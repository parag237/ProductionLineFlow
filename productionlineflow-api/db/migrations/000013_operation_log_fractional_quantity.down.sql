ALTER TABLE operation_work_logs DROP COLUMN unit_of_measure;
ALTER TABLE operation_work_logs
    ALTER COLUMN quantity TYPE BIGINT USING round(quantity)::BIGINT;