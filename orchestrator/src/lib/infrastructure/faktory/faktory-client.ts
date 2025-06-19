import faktory from "faktory-worker";
import type { Job } from "faktory-worker";

const client = new faktory.Client({
  host: process.env.FAKTORY_HOST,
  port: process.env.FAKTORY_PORT,
  password: process.env.FAKTORY_PASSWORD,
});

export { client, faktory };
export type { Job };

