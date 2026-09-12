-- Rolling back the seed removes every rule and category, including ones
-- added later through the UI; transactions keep their rows but lose the
-- category link via ON DELETE SET NULL.
DELETE FROM category_rules;
DELETE FROM categories;
