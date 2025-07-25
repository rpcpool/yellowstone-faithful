import { DataSource } from "@/lib/interfaces/data-source";
import { S3Client } from "@aws-sdk/client-s3";

export interface S3Source extends DataSource {
  // S3Source inherits all methods from DataSource
  // Add S3-specific methods here
  
  bucket: string;
  client: S3Client;
} 