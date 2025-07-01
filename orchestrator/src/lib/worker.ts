import 'dotenv/config';

import { faktory } from '@/lib/infrastructure/faktory/faktory-client';
import { allTasks } from '@/lib/tasks';
import minimist from 'minimist';
import os from 'os';

async function main() {
  const parsedArgs = minimist(process.argv.slice(2));

  // Build queues array based on flags
  const queues = ["default"];
  let workerId: string | null = null;

  // Worker registration function
  async function registerWorker(): Promise<string | null> {
    const workerName = process.env.WORKER_NAME || os.hostname();
    const pid = process.pid;
    
    try {
      const apiUrl = process.env.API_URL || 'http://localhost:3000';
      const response = await fetch(`${apiUrl}/api/workers/register`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          hostname: workerName,
          pid,
          capabilities: ["default", "local"] // Initial capabilities, will add worker ID later
        })
      });

      const data = await response.json();
      
      if (response.status === 200) {
        console.log(`Worker already registered: ${data.source.name}`);
        return data.workerId;
      } else if (response.status === 201) {
        console.log(`Worker registered successfully: ${data.source.name}`);
        return data.workerId;
      } else {
        console.error(`Failed to register worker: ${data.error}`);
        return null;
      }
    } catch (error) {
      console.error('Error registering worker:', error);
      // Continue even if registration fails
      return null;
    }
  }

  // Register worker when running as a source
  if (parsedArgs['local-source'] || parsedArgs['source-id']) {
    const sourceId = parsedArgs['source-id'];
    
    if (sourceId) {
      // If source-id is provided, use it directly
      console.log(`Running as source with ID: ${sourceId}`);
      queues.push(`source.${sourceId}`);
      console.log(`Worker will listen to queues: ${queues.join(', ')}`);
    } else {
      // Legacy behavior for local-source flag
      console.log("Running as local source");
      console.log("Registering worker as local source...");
      workerId = await registerWorker();
      
      if (workerId) {
        queues.push(`source.${workerId}`);
        console.log(`Worker will listen to queues: ${queues.join(', ')}`);
      }
    }
  }

  console.log("Registering tasks:");
  for (const task of Object.values(allTasks())) {
    faktory.register(task.name, task.run);
    console.log(`\t↳ Registered task ${task.name}`);
  }

  await faktory.work({ queues });
}

// Start the worker
main().catch((error: unknown) => {
  console.error(`Worker failed to start: ${error}`);
  process.exit(1);
});