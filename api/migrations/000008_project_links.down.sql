ALTER TABLE projects
    DROP COLUMN IF EXISTS preprod_url,
    DROP COLUMN IF EXISTS prod_url;

ALTER TABLE projects RENAME COLUMN figma_url TO external_url;
