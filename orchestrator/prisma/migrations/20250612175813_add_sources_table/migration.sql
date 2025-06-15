-- CreateEnum
CREATE TYPE "DataSourceType" AS ENUM ('S3', 'HTTP', 'FILESYSTEM', 'OF');

-- CreateTable
CREATE TABLE "sources" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "type" "DataSourceType" NOT NULL,
    "configuration" JSONB NOT NULL,
    "enabled" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "sources_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "sources_name_key" ON "sources"("name");

-- AlterTable
ALTER TABLE "EpochIndex" ADD COLUMN "source_id" TEXT;

-- Create temporary sources for existing data
INSERT INTO "sources" ("id", "name", "type", "configuration", "enabled", "created_at", "updated_at")
VALUES 
    ('clz0h1s0h0000vw4wk5j5j5j5', 'S3', 'S3', '{"bucket": "solana-cars", "region": "us-east-1"}', true, NOW(), NOW()),
    ('clz0h1s0h0001vw4wk5j5j5j6', 'HTTP', 'HTTP', '{"host": "http://localhost:8080", "path": "/cars"}', true, NOW(), NOW()),
    ('clz0h1s0h0002vw4wk5j5j5j7', 'Old Faithful', 'HTTP', '{"host": "https://files.old-faithful.net", "path": ""}', true, NOW(), NOW()),
    ('clz0h1s0h0003vw4wk5j5j5j8', 'Local', 'FILESYSTEM', '{"basePath": "/data/cars"}', true, NOW(), NOW());

-- Update existing EpochIndex records to use source IDs
UPDATE "EpochIndex" SET "source_id" = 
    CASE 
        WHEN "source" = 'S3' THEN 'clz0h1s0h0000vw4wk5j5j5j5'
        WHEN "source" = 'HTTP' THEN 'clz0h1s0h0001vw4wk5j5j5j6'
        WHEN "source" = 'Old Faithful' THEN 'clz0h1s0h0002vw4wk5j5j5j7'
        WHEN "source" = 'Local' THEN 'clz0h1s0h0003vw4wk5j5j5j8'
        ELSE 'clz0h1s0h0001vw4wk5j5j5j6' -- Default to HTTP
    END
WHERE "source_id" IS NULL;

-- Make source_id required
ALTER TABLE "EpochIndex" ALTER COLUMN "source_id" SET NOT NULL;

-- DropIndex
DROP INDEX "EpochIndex_epoch_type_source_key";

-- AlterTable
ALTER TABLE "EpochIndex" DROP COLUMN "source";

-- CreateIndex
CREATE INDEX "EpochIndex_source_id_idx" ON "EpochIndex"("source_id");

-- CreateIndex
CREATE UNIQUE INDEX "EpochIndex_epoch_type_source_id_key" ON "EpochIndex"("epoch", "type", "source_id");

-- AddForeignKey
ALTER TABLE "EpochIndex" ADD CONSTRAINT "EpochIndex_source_id_fkey" FOREIGN KEY ("source_id") REFERENCES "sources"("id") ON DELETE RESTRICT ON UPDATE CASCADE;