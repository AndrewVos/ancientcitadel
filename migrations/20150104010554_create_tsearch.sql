ALTER TABLE urls ADD COLUMN tsv tsvector;

CREATE FUNCTION urls_generate_tsvector() RETURNS trigger AS $$
begin
  new.tsv :=
    setweight(to_tsvector('pg_catalog.english', coalesce(new.title,'')), 'A');
  return new;
end
$$ LANGUAGE plpgsql;

CREATE TRIGGER tsvector_urls_upsert_trigger BEFORE INSERT OR UPDATE
ON urls
FOR EACH ROW EXECUTE PROCEDURE urls_generate_tsvector();

UPDATE urls SET tsv =
setweight(to_tsvector('pg_catalog.english', coalesce(title,'')), 'A');

CREATE INDEX urls_tsv_idx ON urls USING gin(tsv);
