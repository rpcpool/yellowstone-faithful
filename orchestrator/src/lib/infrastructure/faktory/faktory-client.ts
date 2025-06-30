import type { Job } from "faktory-worker";
import faktory from "faktory-worker";

const client = new faktory.Client({
  url: process.env.FAKTORY_URL,
});

export { client, faktory };
export type { Job };

