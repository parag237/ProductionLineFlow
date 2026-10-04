ALTER TABLE operation_work_logs
    ALTER COLUMN quantity TYPE NUMERIC USING quantity::NUMERIC;

ALTER TABLE operation_work_logs
    ADD COLUMN unit_of_measure TEXT NOT NULL DEFAULT 'pieces'
    CHECK (length(btrim(unit_of_measure)) BETWEEN 1 AND 40);

UPDATE operation_work_logs log
SET unit_of_measure = item.unit_of_measure
FROM items item
WHERE item.company_id = log.company_id AND item.id = log.item_id;