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
    ('clz0h1s0h0002vw4wk5j5j5j7', 'Old Faithful', 'HTTP', '{"host": "https://files.old-faithful.net", "path": ""}', true, NOW(), NOW());

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