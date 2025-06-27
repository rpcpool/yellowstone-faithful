-- Update all OF sources to HTTP type with files.old-faithful.net configuration
UPDATE "sources"
SET 
    "type" = 'HTTP',
    "configuration" = jsonb_build_object(
        'host', 'https://files.old-faithful.net',
        'path', ''
    ),
    "updated_at" = CURRENT_TIMESTAMP
WHERE "type" = 'OF';

-- Update the seed data in the initial migration
-- First, remove the old enum value from the type column temporarily
ALTER TABLE "sources" ALTER COLUMN "type" TYPE TEXT;

-- Drop the old enum
DROP TYPE "DataSourceType";

-- Create new enum without OF
CREATE TYPE "DataSourceType" AS ENUM ('S3', 'HTTP', 'FILESYSTEM');

-- Convert the text values back to the new enum
ALTER TABLE "sources" ALTER COLUMN "type" TYPE "DataSourceType" USING "type"::"DataSourceType";