ALTER TABLE items
    ADD COLUMN unit_of_measure TEXT NOT NULL DEFAULT 'pieces'
    CHECK (length(btrim(unit_of_measure)) BETWEEN 1 AND 40);

ALTER TABLE production_runs
    ADD COLUMN unit_of_measure TEXT NOT NULL DEFAULT 'pieces'
    CHECK (length(btrim(unit_of_measure)) BETWEEN 1 AND 40);