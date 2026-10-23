from pathlib import Path
import json
import tempfile
import shutil
import unittest
from unittest.mock import patch

import workflow as w


class LintTests(unittest.TestCase):
    def test_up_only_and_transaction(self):
        text = '-- +goose Up\nSELECT 1;\n-- +goose Down\nDROP TABLE commerce.stores;'
        self.assertEqual(w.up_sql(text), 'BEGIN;\n\nSELECT 1;\n\n\nCOMMIT;\n')

    def test_function_annotations(self):
        text = "-- +goose Up\n-- +goose StatementBegin\nCREATE FUNCTION x() RETURNS text AS $$ SELECT 'a;b' $$ LANGUAGE SQL;\n-- +goose StatementEnd"
        self.assertIn("SELECT 'a;b'", w.up_sql(text))
        for text in ['SELECT 1;', '-- +goose Up\n-- +goose Up',
                     '-- +goose Up\n-- +goose ENVSUB ON',
                     '-- +goose Up\n-- +goose StatementBegin']:
            with self.assertRaises(ValueError):
                w.up_sql(text)

    def test_concurrent_index_matches_goose_transaction(self):
        text = "-- +goose Up\nSET lock_timeout='2s';\nSET statement_timeout='2min';\nCREATE INDEX CONCURRENTLY IF NOT EXISTS probe ON commerce.stores(created_at);"
        self.assertTrue(w.findings(text))
        self.assertEqual(w.findings('-- +goose NO TRANSACTION\n' + text), [])

    def test_down_is_not_linted(self):
        self.assertEqual(w.findings("-- +goose Up\nSET LOCAL lock_timeout='2s';\nSET LOCAL statement_timeout='2min';\nSELECT 1;\n-- +goose Down\nDROP TABLE commerce.stores;"), [])

    def test_rls_change_and_unsafe_ddl(self):
        for text in ['ALTER TABLE commerce.stores DISABLE ROW LEVEL SECURITY;',
                     'DROP TABLE commerce.stores;', 'not valid SQL;']:
            self.assertTrue(w.findings('-- +goose Up\n' + text))

    def test_baseline_and_new_file_gate(self):
        w.lint()
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            old = folder / '00001_old.sql'
            old.write_text('-- +goose Up\nSELECT 1;')
            baseline = folder / 'baseline.json'
            baseline.write_text(json.dumps({old.name: w.digest(old)}))
            w.lint(folder, baseline)
            new = folder / '00002_new.sql'
            new.write_text('-- +goose Up\nDROP TABLE commerce.stores;')
            with self.assertRaises(ValueError):
                w.lint(folder, baseline)
            new.unlink()
            old.write_text('-- +goose Up\nSELECT 2;')
            with self.assertRaises(ValueError):
                w.lint(folder, baseline)
            old.unlink()
            with self.assertRaises(ValueError):
                w.lint(folder, baseline)

    def test_invalid_workspace(self):
        for schema, name in [('commerce; DROP DATABASE postgres', 'safe'),
                             ('commerce', '../escape'), ('commerce', 'UPPER')]:
            with self.assertRaises(ValueError):
                w.workspace(schema, name)

    def test_preserve_edits_and_reject_wrong_scope(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(w, 'WORK', Path(tmp)), \
                patch.dict(w.os.environ, {'REVIEW_SCHEMA': 'commerce', 'REVIEW_NAME': 'guard-test'}):
            folder = w.workspace('commerce', 'guard-test')
            (folder / 'desired.sql').write_text('-- merchant edits\n')
            with patch('sys.argv', ['workflow.py', 'export']), self.assertRaisesRegex(ValueError, 'already exists'):
                w.main()
            (folder / 'scope.json').write_text(json.dumps({'schema': 'auth', 'history': {}}))
            with patch('sys.argv', ['workflow.py', 'draft']), self.assertRaisesRegex(ValueError, 'schema or migration history changed'):
                w.main()
            self.assertEqual((folder / 'desired.sql').read_text(), '-- merchant edits\n')


class RoundTripTests(unittest.TestCase):
    def next_migration(self, folder, name):
        version = max(int(path.name.split('_', 1)[0]) for path in folder.glob('*.sql')) + 1
        return folder / f'{version:05d}_{name}.sql'

    def test_clean_replay_noop_and_generated_column(self):
        with w.replay() as (base, planner), tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            desired = folder / 'desired.sql'
            dumped = w.run(['pgschema', 'dump', *w.flags(base, 'commerce')])
            desired.write_text(dumped)
            w.plan(base, planner, 'commerce', desired, folder)
            self.assertFalse(json.loads((folder / 'plan.json').read_text()).get('groups'))
            (folder / 'baseline.sql').write_text(dumped)
            (folder / 'changes').mkdir()
            (folder / 'changes/store.sql').write_text('ALTER TABLE commerce.stores ADD COLUMN review_label text;\n')
            desired.write_text('\\i baseline.sql\n\\i changes/store.sql\n')
            w.plan(base, planner, 'commerce', desired, folder)
            draft = (folder / 'draft.sql').read_text()
            self.assertIn('review_label', draft)
            # Drafting must not change the source database.
            db = w.connect(base)
            try:
                with db.cursor() as cur:
                    merchant, store = str(w.uuid.uuid4()), str(w.uuid.uuid4())
                    cur.execute('INSERT INTO auth.merchants(id,display_name) VALUES(%s,%s)', (merchant, 'Migration review'))
                    cur.execute('INSERT INTO commerce.stores(merchant_id,id,display_name) VALUES(%s,%s,%s)', (merchant, store, 'Existing store'))
                db.commit()
                with db.cursor() as cur:
                    cur.execute("SELECT count(*) FROM information_schema.columns WHERE table_schema='commerce' AND table_name='stores' AND column_name='review_label'")
                    self.assertEqual(cur.fetchone()[0], 0)
                db.rollback()
                # Review adds explicit scope/timeouts and Goose metadata.
                reviewed = folder / 'reviewed'
                shutil.copytree(w.MIGRATIONS, reviewed)
                migration = self.next_migration(reviewed, 'review')
                migration.write_text("-- +goose Up\nSET LOCAL lock_timeout='2s';\nSET LOCAL statement_timeout='2min';\nSET LOCAL search_path=commerce,pg_catalog,public;\n" + draft)
                w.run(['migration-replay', str(reviewed)], env={**w.os.environ,
                    'MIGRATION_REPLAY_URL': f'postgres://{w.USER}:{w.PASSWORD}@{w.HOST}/{base}?sslmode=disable'})
                with db.cursor() as cur:
                    cur.execute("SELECT count(*) FROM information_schema.columns WHERE table_schema='commerce' AND table_name='stores' AND column_name='review_label'")
                    self.assertEqual(cur.fetchone()[0], 1)
                    cur.execute('SELECT display_name,review_label FROM commerce.stores WHERE merchant_id=%s AND id=%s', (merchant, store))
                    self.assertEqual(cur.fetchone(), ('Existing store', None))
            finally:
                db.close()

    def test_generated_concurrent_index_uses_nontransactional_goose(self):
        with w.replay() as (base, planner), tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            desired = folder / 'desired.sql'
            desired.write_text(w.run(['pgschema', 'dump', *w.flags(base, 'commerce')]) +
                               '\nCREATE INDEX review_created_at ON commerce.stores(created_at);\n')
            w.plan(base, planner, 'commerce', desired, folder)
            draft = (folder / 'draft.sql').read_text()
            self.assertIn('CONCURRENTLY', draft)
            reviewed = folder / 'reviewed'
            shutil.copytree(w.MIGRATIONS, reviewed)
            self.next_migration(reviewed, 'index').write_text('-- +goose NO TRANSACTION\n-- +goose Up\n' +
                "SET lock_timeout='2s';\nSET statement_timeout='2min';\nSET search_path=commerce,pg_catalog,public;\n" + draft)
            env = {**w.os.environ, 'MIGRATION_REPLAY_URL':
                   f'postgres://{w.USER}:{w.PASSWORD}@{w.HOST}/{base}?sslmode=disable'}
            w.run(['migration-replay', str(reviewed)], env=env)
            w.run(['migration-replay', str(reviewed)], env=env)
            db = w.connect(base)
            try:
                with db.cursor() as cur:
                    cur.execute("SELECT indisvalid FROM pg_index WHERE indexrelid='commerce.review_created_at'::regclass")
                    self.assertEqual(cur.fetchone(), (True,))
            finally:
                db.close()


if __name__ == '__main__':
    unittest.main()
