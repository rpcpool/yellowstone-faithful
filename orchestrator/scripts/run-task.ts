import 'dotenv/config';

import { getTask } from '@/lib/tasks';
import minimist from 'minimist';

async function main() {
  const args = process.argv.slice(2);
  const taskName = args[0];
  const taskArgs = args.slice(1);

  if (!taskName) {
    console.error("Usage: npm run task <taskName> [args...]");
    process.exit(1);
  }

  // Parse named arguments using minimist
  const parsedArgs = minimist(taskArgs);

  try {
    const task = getTask(taskName);
    if (!task) {
      console.error(`Unknown task: ${taskName}`);
      process.exit(1);
    }
    if (!task.validateArgs(parsedArgs)) {
      console.error(`Invalid arguments for task ${taskName}:`, JSON.stringify(parsedArgs));
      console.error(`Expected arguments: ${JSON.stringify(task.args)}`);
      process.exit(1);
    }
    await task.run(parsedArgs);
  } catch (error) {
    console.error(`Error running task ${taskName}:`, error);
    process.exit(1);
  }
}

main();
