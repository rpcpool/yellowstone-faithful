-- CreateEnum
CREATE TYPE "IndexType" AS ENUM ('CidToOffsetAndSize', 'SigExists', 'SigToCid', 'SlotToBlocktime', 'SlotToCid');

-- CreateEnum
CREATE TYPE "IndexStatus" AS ENUM ('NotProcessed', 'Processing', 'Present');

-- CreateEnum
CREATE TYPE "EpochStatus" AS ENUM ('NotProcessed', 'Processing', 'Indexed', 'Complete');

-- CreateEnum
CREATE TYPE "JobStatus" AS ENUM ('queued', 'processing', 'completed', 'failed');

-- CreateTable
CREATE TABLE "Epoch" (
    "id" INTEGER NOT NULL,
    "epoch" TEXT NOT NULL,
    "status" "EpochStatus" NOT NULL,
    "cid" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "Epoch_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "EpochIndex" (
    "id" SERIAL NOT NULL,
    "epoch" TEXT NOT NULL,
    "type" "IndexType" NOT NULL,
    "size" BIGINT NOT NULL,
    "status" TEXT NOT NULL,
    "location" TEXT NOT NULL,
    "source" TEXT NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "EpochIndex_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "EpochGsfa" (
    "id" INTEGER NOT NULL,
    "epoch" TEXT NOT NULL,
    "exists" BOOLEAN DEFAULT false,
    "location" TEXT NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "EpochGsfa_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "jobs" (
    "id" TEXT NOT NULL,
    "epoch_id" INTEGER NOT NULL,
    "job_type" TEXT NOT NULL,
    "status" "JobStatus" NOT NULL,
    "metadata" JSONB,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "jobs_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "Epoch_epoch_key" ON "Epoch"("epoch");

-- CreateIndex
CREATE UNIQUE INDEX "EpochIndex_epoch_type_source_key" ON "EpochIndex"("epoch", "type", "source");

-- AddForeignKey
ALTER TABLE "EpochIndex" ADD CONSTRAINT "EpochIndex_epoch_fkey" FOREIGN KEY ("epoch") REFERENCES "Epoch"("epoch") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "EpochGsfa" ADD CONSTRAINT "EpochGsfa_epoch_fkey" FOREIGN KEY ("epoch") REFERENCES "Epoch"("epoch") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "jobs" ADD CONSTRAINT "jobs_epoch_id_fkey" FOREIGN KEY ("epoch_id") REFERENCES "Epoch"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
