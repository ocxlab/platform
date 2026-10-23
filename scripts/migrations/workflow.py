"""Local review tooling. Database targets are deliberately not configurable."""
import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import uuid

import psycopg2
from psycopg2 import sql

ROOT = Path('/workspace')
MIGRATIONS = ROOT / 'services/platform/internal/infra/database/migrations'
BASELINE = ROOT / 'scripts/migrations/baseline.json'
WORK = ROOT / 'working/migrations'
PASSWORD = 'migration_review_local_only'
HOST = 'migration-db'
USER = 'migration_review'
SCHEMAS = {'auth', 'commerce', 'runtime', 'discovery', 'payment',
           'commerce_internal', 'shopping', 'lodging', 'payments'}


def run(args, **kwargs):
    result = subprocess.run(args, text=True, capture_output=True, timeout=180, **kwargs)
    if result.returncode:
        raise RuntimeError(f'{args[0]} failed ({result.returncode}): {result.stderr}')
    return result.stdout


def up_sql(text):
    """Goose's own-line annotations are lexical, including inside StatementBegin.

    Preserve line positions for findings. Refuse env substitution rather than
    checking SQL different from the text Goose would execute.
    """
    active = False
    seen = False
    down = False
    transactional = True
    block = False
    lines = []
    for line in text.splitlines():
        match = re.fullmatch(r'--\s+\+goose\s+(.+?)\s*', line, re.I)
        if match:
            tag = match[1].lower()
            if tag == 'up':
                if seen or down:
                    raise ValueError('exactly one Up annotation required')
                active = seen = True
            elif tag == 'down':
                if not seen or down or block:
                    raise ValueError('invalid Down annotation')
                active = False
                down = True
            elif tag == 'no transaction':
                transactional = False
            elif tag.startswith('envsub'):
                raise ValueError('environment substitution is not supported by this review gate')
            elif tag == 'statementbegin':
                if block:
                    raise ValueError('nested StatementBegin')
                block = True
            elif tag == 'statementend':
                if not block:
                    raise ValueError('unmatched StatementEnd')
                block = False
            else:
                raise ValueError(f'unsupported Goose annotation: {tag}')
            lines.append('')
        else:
            lines.append(line if active else '')
    if not seen or block:
        raise ValueError('missing Up or unclosed StatementBegin')
    body = '\n'.join(lines)
    return ('BEGIN;\n' + body + '\nCOMMIT;\n') if transactional else body


def digest(path):
    # Stable on Windows CRLF and Linux LF checkouts.
    return hashlib.sha256(path.read_text().encode()).hexdigest()


def findings(text):
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / 'up.sql'
        path.write_text(up_sql(text))
        result = subprocess.run(['squawk', '--pg-version=18.0', '--reporter=json',
                                 '--include=ban-alter-row-level-security,require-table-schema',
                                 str(path)], text=True, capture_output=True, timeout=30)
        if result.returncode not in (0, 1):
            raise RuntimeError(result.stderr or result.stdout)
        data = json.loads(result.stdout)
        # Syntax/parse errors are findings, never a clean run.
        if result.returncode and not data:
            raise RuntimeError(result.stderr or 'Squawk failed without findings')
        return data


def lint(directory=MIGRATIONS, baseline_file=BASELINE):
    baseline = json.loads(baseline_file.read_text())
    paths = {p.name: p for p in directory.glob('*.sql')}
    for name, expected in baseline.items():
        if name not in paths or digest(paths[name]) != expected:
            raise ValueError(f'historical migration changed or missing: {name}')
    failed = False
    for name, path in sorted(paths.items()):
        if name in baseline:
            continue
        if int(name.split('_', 1)[0]) <= max(int(n.split('_', 1)[0]) for n in baseline):
            raise ValueError('new migrations must follow the historical baseline')
        issues = findings(path.read_text())
        if issues:
            print(name, json.dumps(issues, indent=2))
            failed = True
    if failed:
        raise ValueError('new migrations failed Squawk review')
    print(f'Historical hashes verified: {len(baseline)}; new migrations passed Squawk')


def connect(db='postgres'):
    return psycopg2.connect(host=HOST, user=USER, password=PASSWORD, dbname=db)


@contextmanager
def replay():
    """Unique databases on the dedicated, unexposed migration-db only."""
    base = 'review_' + uuid.uuid4().hex
    planner = base + '_planner'
    admin = connect()
    admin.autocommit = True
    try:
        with admin.cursor() as cur:
            cur.execute(sql.SQL('CREATE DATABASE {}').format(sql.Identifier(base)))
        run(['migration-replay', str(MIGRATIONS)], env={**os.environ,
            'MIGRATION_REPLAY_URL': f'postgres://{USER}:{PASSWORD}@{HOST}/{base}?sslmode=disable'})
        with admin.cursor() as cur:
            cur.execute(sql.SQL('CREATE DATABASE {} TEMPLATE {}').format(
                sql.Identifier(planner), sql.Identifier(base)))
        yield base, planner
    finally:
        with admin.cursor() as cur:
            for name in (planner, base):
                cur.execute(sql.SQL('DROP DATABASE IF EXISTS {} WITH (FORCE)').format(sql.Identifier(name)))
        admin.close()


def flags(db, schema):
    return ['--host', HOST, '--user', USER, '--password', PASSWORD,
            '--sslmode', 'disable', '--db', db, '--schema', schema]


def plan(base, planner, schema, desired, folder):
    return run(['pgschema', 'plan', *flags(base, schema), '--file', str(desired),
                '--output-sql', str(folder / 'draft.sql'),
                '--output-json', str(folder / 'plan.json'),
                '--plan-host', HOST, '--plan-db', planner, '--plan-user', USER,
                '--plan-password', PASSWORD, '--plan-sslmode', 'disable'])


def workspace(schema, name):
    if schema not in SCHEMAS or not re.fullmatch(r'[a-z][a-z0-9_-]{0,63}', name):
        raise ValueError('use a known schema and a simple lowercase NAME')
    folder = WORK / name
    if folder.resolve().parent != WORK.resolve():
        raise ValueError('workspace symlinks are not permitted')
    folder.mkdir(parents=True, exist_ok=True)
    return folder


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('command', choices=['lint', 'export', 'draft'])
    args = parser.parse_args()
    if args.command == 'lint':
        lint()
        return
    schema = os.environ.get('REVIEW_SCHEMA', 'commerce')
    folder = workspace(schema, os.environ.get('REVIEW_NAME', 'schema'))
    desired = folder / 'desired.sql'
    metadata = folder / 'scope.json'
    history = {p.name: digest(p) for p in sorted(MIGRATIONS.glob('*.sql'))}
    scope = {'schema': schema, 'history': history}
    for name in ('scope.json', 'desired.sql', 'draft.sql', 'plan.json'):
        if (folder / name).is_symlink():
            raise ValueError('workspace file symlinks are not permitted')
    if args.command == 'export' and desired.exists():
        raise ValueError('desired.sql already exists; choose a new NAME to preserve your edits')
    if args.command == 'draft' and (not desired.is_file() or desired.is_symlink()):
        raise ValueError('export and edit desired.sql first')
    if args.command == 'draft' and (not metadata.is_file() or json.loads(metadata.read_text()) != scope):
        raise ValueError('schema or migration history changed; export a new NAME and merge your desired edits')
    with replay() as (base, planner):
        if args.command == 'export':
            with desired.open('x') as out:
                out.write(run(['pgschema', 'dump', *flags(base, schema)]))
            metadata.write_text(json.dumps(scope, indent=2) + '\n')
        else:
            plan(base, planner, schema, desired, folder)
            print('Review draft.sql and plan.json. No product database was changed.')
    print(folder.relative_to(ROOT))


if __name__ == '__main__':
    main()
