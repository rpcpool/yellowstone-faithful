'use server';

import { JobStatus } from '@/generated/prisma';
import { getLatestJob, getRecentJobs, storeJobRecord, updateJobStatus } from '@/lib/jobs';
import { getTask } from '@/lib/tasks';

export async function scheduleTask(taskName: string, args: Record<string, unknown>) {
  const task = getTask(taskName);

  if (!task) {
    return {
      success: false,
      message: `Task ${taskName} not found`,
    };
  }

  // Ensure the provided arguments are valid for the given task
  if (!task.validateArgs(args)) {
    return {
      success: false,
      message: `Invalid arguments supplied for task: ${task.name}`,
    };
  }

  let jobId: string | undefined;
  const epochId: number = typeof args?.epochId === 'number' ? args.epochId : -1;

  try {
    // Delegate the actual scheduling logic to the task implementation
    jobId = await task.schedule(args);

    // Record job in the database as queued
    await storeJobRecord({
      id: jobId,
      epochId,
      jobType: task.name,
      status: JobStatus.queued,
      metadata: {
        args,
        triggeredBy: 'manual',
        timestamp: new Date().toISOString(),
      },
    });

    return {
      success: true,
      message: `Task ${task.name} has been scheduled successfully.`,
      jobId,
      taskName: task.name,
    };
  } catch (error) {
    console.error(`Failed to schedule task ${task.name}:`, error);

    // If we already obtained a job ID, record the failure status
    if (jobId) {
      try {
        await updateJobStatus(jobId, JobStatus.failed);
      } catch (updateError) {
        console.error('Failed to update job status to failed:', updateError);
      }
    }

    return {
      success: false,
      message: `Failed to schedule task: ${task.name}`,
      error: error instanceof Error ? error.message : 'Unknown error',
    };
  }
}

// Function to get recent GSFA job statuses
export async function getGSFAJobStatus(epochId: number) {
  const jobs = await getRecentJobs(epochId, 'BuildGSFAIndex');
  return {
    success: true,
    jobs: jobs.map(job => ({
      id: job.id,
      status: job.status,
      createdAt: job.createdAt,
      updatedAt: job.updatedAt,
      metadata: job.metadata,
    })),
  };
}

// Function to schedule a refresh epoch job
export async function scheduleRefreshEpochJob(epochId: number) {
  if (isNaN(epochId) || epochId < 0) {
    return {
      success: false,
      message: 'Invalid epoch ID'
    };
  }

  try {
    // Check if there's already a queued or processing refresh job for this epoch
    const existingJob = await checkExistingRefreshJob(epochId);
    if (existingJob) {
      return {
        success: false,
        message: `Refresh job is already ${existingJob.status} for epoch ${epochId}`,
        jobId: existingJob.id,
        status: existingJob.status
      };
    }

    const faktoryUrl = process.env.FAKTORY_URL || 'tcp://localhost:7419';
    console.log('Scheduling refresh epoch job via Faktory...', { faktoryUrl, epochId });
    
    // Check if faktory-worker is available
    let faktory;
    try {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      faktory = require('faktory-worker');
    } catch (requireError) {
      console.error('faktory-worker package not found:', requireError);
      return {
        success: false,
        message: 'faktory-worker package is not installed',
        error: 'Missing dependency: faktory-worker'
      };
    }
    
    // Generate unique job ID
    const jobId = `refresh-${epochId}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    
    // Connect to Faktory server
    let client;
    try {
      client = await faktory.connect({
        url: faktoryUrl
      });
    } catch (connectionError) {
      console.error('Failed to connect to Faktory server:', connectionError);
      return {
        success: false,
        message: `Failed to connect to Faktory server at ${faktoryUrl}`,
        error: connectionError instanceof Error ? connectionError.message : 'Connection failed',
        details: 'Make sure Faktory server is running and FAKTORY_URL is correctly configured'
      };
    }
    
    try {
      // Store job record in database before scheduling
      await storeJobRecord({
        id: jobId,
        epochId,
        jobType: 'RefreshEpoch',
        status: JobStatus.queued,
        metadata: {
          epochId: epochId.toString(),
          triggeredBy: 'manual',
          timestamp: new Date().toISOString()
        }
      });
      
      // Schedule the RefreshEpoch job with tracking ID
      await client.job('RefreshEpoch', { 
        jobId, // Add job ID for tracking
        epochId: epochId.toString(),
        triggeredBy: 'manual',
        timestamp: new Date().toISOString()
      }).push();
      
      await client.close();
      
      return {
        success: true,
        message: `Refresh job has been scheduled successfully for epoch ${epochId}. The process will scan all data sources and update the epoch status.`,
        jobType: 'RefreshEpoch',
        jobId,
        epochId,
        faktoryUrl
      };
    } catch (jobError) {
      console.error('Failed to schedule refresh epoch job:', jobError);
      await client.close();
      
      // Update job status to failed if we stored it
      try {
        await updateJobStatus(jobId, JobStatus.failed);
      } catch (updateError) {
        console.error('Failed to update job status to failed:', updateError);
      }
      
      return {
        success: false,
        message: 'Failed to schedule refresh epoch job in Faktory',
        error: jobError instanceof Error ? jobError.message : 'Job scheduling failed'
      };
    }
  } catch (error) {
    console.error('Unexpected error scheduling refresh epoch job:', error);
    return {
      success: false,
      message: 'Unexpected error occurred',
      error: error instanceof Error ? error.message : 'Unknown error'
    };
  }
}

// Helper function to check for existing refresh jobs
export async function checkExistingRefreshJob(epochId: number) {
  return getLatestJob(epochId, 'RefreshEpoch', [JobStatus.queued, JobStatus.processing]);
}

// Function to get refresh job status
export async function getRefreshJobStatus(epochId: number) {
  const jobs = await getRecentJobs(epochId, 'RefreshEpoch');
  return {
    success: true,
    jobs: jobs.map(job => ({
      id: job.id,
      status: job.status,
      createdAt: job.createdAt,
      updatedAt: job.updatedAt,
      metadata: job.metadata,
    })),
  };
} 