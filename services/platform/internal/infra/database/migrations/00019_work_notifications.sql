-- +goose Up
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '2min';

-- Wake hints contain only the bounded domain name. PostgreSQL delivers NOTIFY
-- on commit; the durable row, never the notification, establishes required work.
-- +goose StatementBegin
CREATE FUNCTION runtime.notify_work_change() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 PERFORM pg_notify('ocp_work_changed',NEW.domain);
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION runtime.notify_work_change() FROM PUBLIC;
CREATE TRIGGER work_change AFTER INSERT OR UPDATE OF state,available_at,lease_until ON runtime.effects
FOR EACH ROW EXECUTE FUNCTION runtime.notify_work_change();

-- Only a relative delay is exposed; no tenant identifiers or work payloads.
-- Pending retries and abandoned leases both need a timer without another insert.
-- +goose StatementBegin
CREATE FUNCTION runtime.next_effect_delay(domains text[]) RETURNS double precision
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE due timestamptz;
BEGIN
 IF domains IS NULL OR cardinality(domains) NOT BETWEEN 1 AND 3
 OR array_position(domains,NULL) IS NOT NULL
 OR NOT domains <@ ARRAY['food','shopping','lodging']::text[]
 THEN RAISE EXCEPTION 'invalid work schedule domains'; END IF;
 SELECT min(CASE WHEN e.state='pending' THEN e.available_at ELSE e.lease_until END)
 INTO due FROM runtime.effects e WHERE e.domain=ANY(domains)
 AND ((e.state='pending' AND e.attempts<20) OR e.state='leased');
 IF due IS NULL THEN RETURN -1; END IF;
 RETURN greatest(0,extract(epoch FROM due-clock_timestamp()))::double precision;
END;
$$;
-- +goose StatementEnd
ALTER FUNCTION runtime.next_effect_delay(text[]) OWNER TO ocp_work_owner;
REVOKE ALL ON FUNCTION runtime.next_effect_delay(text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION runtime.next_effect_delay(text[]) TO ocp_app;
