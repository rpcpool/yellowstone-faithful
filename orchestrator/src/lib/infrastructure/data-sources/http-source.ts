import { DataSource } from "@/lib/interfaces/data-source";

export interface HTTPSource extends DataSource {
  host: string;
  path: string;
  basicAuth?: {
    username: string;
    password: string;
  };
}