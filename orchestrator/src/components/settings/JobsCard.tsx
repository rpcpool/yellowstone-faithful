import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

interface Job {
  id: string;
  epochId: number;
  jobType: string;
  status: string;
  createdAt: string;
  updatedAt: string;
}

interface Pagination {
  page: number;
  pageSize: number;
  totalCount: number;
  totalPages: number;
}

interface JobsCardProps {
  epochIdFilter: string;
  setEpochIdFilter: (value: string) => void;
  jobTypeFilter: string;
  setJobTypeFilter: (value: string) => void;
  currentPage: number;
  setCurrentPage: (page: number) => void;
  jobs: Job[];
  isLoadingJobs: boolean;
  jobsError: unknown;
  pagination?: Pagination;
  deleteJobMutation: {
    mutate: (id: string) => void;
    isPending: boolean;
  };
}

export function JobsCard({
  epochIdFilter,
  setEpochIdFilter,
  jobTypeFilter,
  setJobTypeFilter,
  currentPage,
  setCurrentPage,
  jobs,
  isLoadingJobs,
  jobsError,
  pagination,
  deleteJobMutation,
}: JobsCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Jobs</CardTitle>
        <CardDescription>All tracked jobs in the system</CardDescription>
      </CardHeader>
      <CardContent>
        {/* Filters */}
        <div className="flex items-end gap-4 mb-4">
          <div className="flex-1 flex flex-col gap-1">
            <Label htmlFor="epochId">Filter by Epoch ID</Label>
            <Input
              id="epochId"
              type="number"
              placeholder="Enter epoch ID"
              value={epochIdFilter}
              onChange={(e) => setEpochIdFilter(e.target.value)}
            />
          </div>
          <div className="flex-1 flex flex-col gap-1">
            <Label htmlFor="jobType">Filter by Job Type</Label>
            <Select value={jobTypeFilter || undefined} onValueChange={(value) => setJobTypeFilter(value || "")}> 
              <SelectTrigger>
                <SelectValue placeholder="All job types" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="BuildGSFAIndex">Build GSFA Index</SelectItem>
                <SelectItem value="RefreshEpoch">Refresh Epoch</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-end">
            <Button 
              variant="outline" 
              onClick={() => {
                setEpochIdFilter("");
                setJobTypeFilter("");
                setCurrentPage(1);
              }}
            >
              Clear Filters
            </Button>
          </div>
        </div>

        {isLoadingJobs ? (
          <p className="text-sm text-muted-foreground">Loading...</p>
        ) : jobsError ? (
          <p className="text-sm text-red-500">{(jobsError as Error).message}</p>
        ) : jobs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No jobs found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left border-b">
                  <th className="py-2 pr-2">ID</th>
                  <th className="py-2 pr-2">Epoch</th>
                  <th className="py-2 pr-2">Type</th>
                  <th className="py-2 pr-2">Status</th>
                  <th className="py-2 pr-2">Created</th>
                  <th className="py-2 pr-2">Updated</th>
                  <th className="py-2 pr-2" />
                </tr>
              </thead>
              <tbody>
                {jobs.map((job: Job) => (
                  <tr key={job.id} className="border-b hover:bg-muted/50">
                    <td className="py-2 pr-2 font-mono text-xs">{job.id}</td>
                    <td className="py-2 pr-2">{job.epochId}</td>
                    <td className="py-2 pr-2">{job.jobType}</td>
                    <td className="py-2 pr-2 capitalize">{job.status}</td>
                    <td className="py-2 pr-2">
                      {new Date(job.createdAt).toLocaleString()}
                    </td>
                    <td className="py-2 pr-2">
                      {new Date(job.updatedAt).toLocaleString()}
                    </td>
                    <td className="py-2 pr-2 text-right">
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => deleteJobMutation.mutate(job.id)}
                        disabled={deleteJobMutation.isPending}
                      >
                        Delete
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {pagination && pagination.totalPages > 1 && (
          <div className="flex items-center justify-between mt-4">
            <div className="text-sm text-muted-foreground">
              Showing {((pagination.page - 1) * pagination.pageSize) + 1} to{" "}
              {Math.min(pagination.page * pagination.pageSize, pagination.totalCount)} of{" "}
              {pagination.totalCount} jobs
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(currentPage - 1)}
                disabled={currentPage <= 1}
              >
                Previous
              </Button>
              <span className="text-sm">
                Page {pagination.page} of {pagination.totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(currentPage + 1)}
                disabled={currentPage >= pagination.totalPages}
              >
                Next
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
