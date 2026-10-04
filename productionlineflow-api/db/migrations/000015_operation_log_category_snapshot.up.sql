ALTER TABLE operation_work_logs
    ADD COLUMN category_name TEXT NOT NULL DEFAULT '';

UPDATE operation_work_logs log
SET category_name = category.name
FROM items item
JOIN item_categories category
  ON category.company_id = item.company_id
 AND category.id = item.category_id
WHERE item.company_id = log.company_id
  AND item.id = log.item_id;