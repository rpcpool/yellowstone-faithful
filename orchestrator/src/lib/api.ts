interface JobsApiResponse {
  success: boolean;
  jobs: Array<{
    id: string;
    epochId: number;
    jobType: string;
    status: string;
    createdAt: string;
    updatedAt: string;
    metadata?: Record<string, unknown>;
  }>;
  filters: {
    epochId: number | null;
    jobType: string | null;
  };
  pagination: {
    page: number;
    pageSize: number;
    totalCount: number;
    totalPages: number;
  };
}

interface JobsApiParams {
  page?: number;
  pageSize?: number;
  epochId?: number;
  jobType?: string;
}

export async function fetchJobs(params: JobsApiParams = {}): Promise<JobsApiResponse> {
  const searchParams = new URLSearchParams();
  
  if (params.page) searchParams.set('page', params.page.toString());
  if (params.pageSize) searchParams.set('pageSize', params.pageSize.toString());
  if (params.epochId) searchParams.set('epochId', params.epochId.toString());
  if (params.jobType) searchParams.set('jobType', params.jobType);

  const url = `/api/jobs${searchParams.toString() ? `?${searchParams.toString()}` : ''}`;
  
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to fetch jobs: ${response.statusText}`);
  }
  
  return response.json();
}

export async function deleteJob(jobId: string): Promise<{ success: boolean; error?: string }> {
  const response = await fetch(`/api/jobs/${jobId}`, {
    method: 'DELETE',
  });
  
  if (!response.ok) {
    throw new Error(`Failed to delete job: ${response.statusText}`);
  }
  
  return response.json();
} 
interface IndexesApiResponse {
  success: boolean;
  indexes: Array<{
    id: number;
    epoch: string;
    type: string;
    source: string;
    size: string;
    status: string;
    location: string;
    createdAt: string;
    updatedAt: string;
  }>;
  filters: {
    source: string | null;
    type: string | null;
    search: string | null;
  };
  availableSources: string[];
  availableTypes: string[];
  pagination: {
    page: number;
    pageSize: number;
    totalCount: number;
    totalPages: number;
  };
}

interface IndexesApiParams {
  page?: number;
  pageSize?: number;
  source?: string;
  type?: string;
  search?: string;
  sortBy?: string;
  sortOrder?: string;
}

export async function fetchIndexes(params: IndexesApiParams = {}): Promise<IndexesApiResponse> {
  const searchParams = new URLSearchParams();

  if (params.page) searchParams.set('page', params.page.toString());
  if (params.pageSize) searchParams.set('pageSize', params.pageSize.toString());
  if (params.source) searchParams.set('source', params.source);
  if (params.type) searchParams.set('type', params.type);
  if (params.search) searchParams.set('search', params.search);
  if (params.sortBy) searchParams.set('sortBy', params.sortBy);
  if (params.sortOrder) searchParams.set('sortOrder', params.sortOrder);

  const url = `/api/indexes${searchParams.toString() ? `?${searchParams.toString()}` : ''}`;

  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to fetch indexes: ${response.statusText}`);
  }

  return response.json();
}
