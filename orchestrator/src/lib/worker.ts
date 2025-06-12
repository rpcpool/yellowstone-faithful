import 'dotenv/config';

import { faktory } from '@/lib/infrastructure/faktory/faktory-client';
import { allTasks } from '@/lib/tasks';
import minimist from 'minimist';


const parsedArgs = minimist(process.argv.slice(2));

// Build queues array based on flags
const queues = ["default"];

if (parsedArgs.local) {
  console.log("Running in local mode");
  queues.push("local");
}

console.log("Registering tasks:");
for (const task of Object.values(allTasks())) {
  faktory.register(task.name, task.run);
  console.log(`\t↳ Registered task ${task.name}`);
}

faktory.work(
  {
    queues
  }
).catch((error: unknown) => {
  console.error(`worker failed to start: ${error}`);
  process.exit(1);
});