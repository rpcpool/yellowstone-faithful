import { NextRequest, NextResponse } from "next/server";
import { getTask } from "@/lib/tasks";
import { z } from "zod";

const scheduleJobSchema = z.object({
  jobType: z.string(),
  params: z.record(z.any()).optional(),
});

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    
    // Validate request body
    const parseResult = scheduleJobSchema.safeParse(body);
    if (!parseResult.success) {
      return NextResponse.json(
        {
          success: false,
          error: "Invalid request body",
          details: parseResult.error.errors,
        },
        { status: 400 }
      );
    }

    const { jobType, params = {} } = parseResult.data;

    // Get the task
    const task = getTask(jobType);
    if (!task) {
      return NextResponse.json(
        {
          success: false,
          error: `Unknown job type: ${jobType}`,
        },
        { status: 400 }
      );
    }

    // Validate task arguments if the task has validation
    if (task.validateArgs && !task.validateArgs(params)) {
      return NextResponse.json(
        {
          success: false,
          error: "Invalid job parameters",
          details: `Parameters do not match the schema for job type: ${jobType}`,
        },
        { status: 400 }
      );
    }

    // Schedule the job
    if (!task.schedule) {
      return NextResponse.json(
        {
          success: false,
          error: `Job type ${jobType} does not support scheduling`,
        },
        { status: 400 }
      );
    }

    const jobId = await task.schedule(params);

    return NextResponse.json({
      success: true,
      jobId,
      message: `Job ${jobType} scheduled successfully`,
    });

  } catch (error) {
    console.error("Error scheduling job:", error);
    
    // Check if it's a Faktory connection error
    if (error instanceof Error && error.message.includes("ECONNREFUSED")) {
      return NextResponse.json(
        {
          success: false,
          error: "Failed to connect to Faktory",
          details: "Please ensure Faktory is running",
        },
        { status: 503 }
      );
    }

    return NextResponse.json(
      {
        success: false,
        error: "Failed to schedule job",
        details: error instanceof Error ? error.message : "Unknown error",
      },
      { status: 500 }
    );
  }
}