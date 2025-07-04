"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PaginationWithLinks } from "@/components/ui/pagination-with-links";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useQuery } from "@tanstack/react-query";
import { Search, X } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

interface Epoch {
  id: number;
  epoch: string;
  status: "NotProcessed" | "Indexed" | "Complete";
  createdAt: string;
  updatedAt: string;
}

interface EpochsResponse {
  epochs: Epoch[];
  pagination: {
    page: number;
    pageSize: number;
    totalCount: number;
    totalPages: number;
  };
}

async function getEpochs(page: number, pageSize: number, search?: string, status?: string): Promise<EpochsResponse> {
  const params = new URLSearchParams({
    page: page.toString(),
    pageSize: pageSize.toString(),
  });
  
  if (search && search.trim()) {
    params.append('search', search.trim());
  }
  
  if (status && status !== 'all') {
    params.append('status', status);
  }
  
  const response = await fetch(
    `/api/epochs?${params.toString()}`,
    {
      cache: 'no-store',
    }
  );

  if (!response.ok) {
    throw new Error('Failed to fetch epochs');
  }

  return response.json();
}

function getStatusColor(status: Epoch['status']) {
  switch (status) {
    case 'Complete':
      return 'bg-green-500/10 text-green-500 border-green-500/20 dark:bg-green-500/20 dark:text-green-400 dark:border-green-500/30';
    case 'Indexed':
      return 'bg-blue-500/10 text-blue-500 border-blue-500/20 dark:bg-blue-500/20 dark:text-blue-400 dark:border-blue-500/30';
    case 'NotProcessed':
      return 'bg-gray-500/10 text-gray-700 border-gray-500/20 dark:bg-gray-500/20 dark:text-gray-300 dark:border-gray-500/30';
    default:
      return 'bg-gray-500/10 text-gray-700 border-gray-500/20 dark:bg-gray-500/20 dark:text-gray-300 dark:border-gray-500/30';
  }
}

function TableSkeleton() {
  return (
    <div className="space-y-4">
      {/* Stats skeleton */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-4 w-24" />
      </div>
      
      {/* Table skeleton */}
      <div className="border rounded-lg">
        <div className="overflow-hidden">
          {/* Header */}
          <div className="grid grid-cols-5 gap-4 p-4 border-b bg-muted/50 font-medium text-sm">
            <div>ID</div>
            <div>Epoch</div>
            <div>Status</div>
            <div>Created</div>
            <div>Updated</div>
          </div>
          
          {/* Rows */}
          <div className="divide-y">
            {Array.from({ length: 10 }).map((_, i) => (
              <div key={i} className="grid grid-cols-5 gap-4 p-4">
                <Skeleton className="h-4 w-8" />
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-6 w-20" />
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-20" />
              </div>
            ))}
          </div>
        </div>
      </div>
      
      {/* Pagination skeleton */}
      <div className="flex justify-center">
        <Skeleton className="h-10 w-80" />
      </div>
    </div>
  );
}

function EpochsTable({ epochs }: { epochs: Epoch[] }) {
  if (epochs.length === 0) {
    return (
      <div className="border rounded-lg">
        <div className="grid grid-cols-5 gap-4 p-4 border-b bg-muted/50 font-medium text-sm">
          <div>ID</div>
          <div>Epoch</div>
          <div>Status</div>
          <div>Created</div>
          <div>Updated</div>
        </div>
        <div className="p-12 text-center">
          <div className="text-muted-foreground">
            <h3 className="text-lg font-medium mb-2 text-foreground">No epochs found</h3>
            <p className="text-sm">No epochs match your current filters.</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="grid grid-cols-5 gap-4 p-4 border-b bg-muted/50 font-medium text-sm">
        <div>ID</div>
        <div>Epoch</div>
        <div>Status</div>
        <div>Created</div>
        <div>Updated</div>
      </div>
      
      {/* Rows */}
      <div className="divide-y">
        {epochs.map((epoch) => (
          <div 
            key={epoch.id} 
            className="grid grid-cols-5 gap-4 p-4 hover:bg-muted/50 transition-colors"
          >
            <div>
              <Link 
                href={`/epochs/${epoch.id}`}
                className="font-mono text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 hover:underline font-medium"
              >
                {epoch.id}
              </Link>
            </div>
            <div>
              <span className="font-mono text-sm text-foreground">{epoch.epoch}</span>
            </div>
            <div>
              <Badge className={getStatusColor(epoch.status)}>
                {epoch.status}
              </Badge>
            </div>
            <div className="text-sm text-muted-foreground">
              {new Date(epoch.createdAt).toLocaleDateString()}
            </div>
            <div className="text-sm text-muted-foreground">
              {new Date(epoch.updatedAt).toLocaleDateString()}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// Custom hook for debounced value
function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value);

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => {
      clearTimeout(handler);
    };
  }, [value, delay]);

  return debouncedValue;
}

export default function EpochsPage() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  
  // Get initial values from URL params
  const initialPage = parseInt(searchParams.get('page') || '1', 10);
  const initialPageSize = parseInt(searchParams.get('pageSize') || '10', 10);
  const initialSearch = searchParams.get('search') || '';
  const initialStatus = searchParams.get('status') || 'all';
  
  // State
  const [page, setPage] = useState(initialPage);
  const [pageSize, setPageSize] = useState(initialPageSize);
  const [search, setSearch] = useState(initialSearch);
  const [status, setStatus] = useState(initialStatus);
  
  // Debounce search to avoid too many API calls
  const debouncedSearch = useDebounce(search, 300);
  
  // Update URL when filters change
  const updateURL = useCallback((newPage: number, newSearch: string, newStatus: string, newPageSize: number) => {
    const params = new URLSearchParams();
    params.set('page', newPage.toString());
    params.set('pageSize', newPageSize.toString());
    
    if (newSearch.trim()) {
      params.set('search', newSearch.trim());
    }
    
    if (newStatus !== 'all') {
      params.set('status', newStatus);
    }
    
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }, [router, pathname]);
  
  const {
    data,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['epochs', page, pageSize, debouncedSearch, status],
    queryFn: () => getEpochs(page, pageSize, debouncedSearch, status),
    placeholderData: (previousData) => previousData,
  });
  
  // Main effect to fetch data based on current URL parameters
  useEffect(() => {
    const urlPage = parseInt(searchParams.get('page') || '1', 10);
    const urlPageSize = parseInt(searchParams.get('pageSize') || '10', 10);
    const urlSearch = searchParams.get('search') || '';
    const urlStatus = searchParams.get('status') || 'all';
    
    // Update local state to match URL (without triggering other effects)
    setPage(urlPage);
    setPageSize(urlPageSize);
    setSearch(urlSearch);
    setStatus(urlStatus);
    
  }, [searchParams]);
  
  // Effect for debounced search changes
  useEffect(() => {
    const currentSearch = searchParams.get('search') || '';
    if (debouncedSearch !== currentSearch && debouncedSearch !== initialSearch) {
      updateURL(1, debouncedSearch, status, pageSize);
    }
  }, [debouncedSearch, searchParams, status, pageSize, updateURL, initialSearch]);
  
  
  const handleSearchChange = (value: string) => {
    setSearch(value);
  };
  
  const handleStatusChange = (value: string) => {
    setStatus(value);
    updateURL(1, debouncedSearch, value, pageSize);
  };
  
  const handlePageSizeChange = (value: string) => {
    const newPageSize = parseInt(value, 10);
    setPageSize(newPageSize);
    updateURL(1, debouncedSearch, status, newPageSize);
  };
  
  const clearFilters = () => {
    setSearch('');
    setStatus('all');
    updateURL(1, '', 'all', pageSize);
  };
  
  const hasActiveFilters = search.trim() || status !== 'all';

  return (
    <div className="px-4 py-8 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight text-foreground">Epochs</h1>
        <p className="text-muted-foreground mt-2">
          Browse and manage epoch data processing status
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground h-4 w-4" />
          <Input
            placeholder="Search by ID or epoch name..."
            value={search}
            onChange={(e) => handleSearchChange(e.target.value)}
            className="pl-10 pr-10"
          />
          {search && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleSearchChange('')}
              className="absolute right-1 top-1/2 transform -translate-y-1/2 h-6 w-6 p-0"
            >
              <X className="h-3 w-3" />
            </Button>
          )}
        </div>
        
        <div className="flex gap-2">
          <Select value={status} onValueChange={handleStatusChange}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Filter by status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Statuses</SelectItem>
              <SelectItem value="NotProcessed">Not Processed</SelectItem>
              <SelectItem value="Indexed">Indexed</SelectItem>
              <SelectItem value="Complete">Complete</SelectItem>
            </SelectContent>
          </Select>
          
          {hasActiveFilters && (
            <Button variant="outline" onClick={clearFilters}>
              Clear Filters
            </Button>
          )}
        </div>
      </div>

      {/* Content */}
      {isLoading ? (
        <TableSkeleton />
      ) : error ? (
        <div className="flex items-center justify-center py-12">
          <div className="text-center">
            <h3 className="text-lg font-medium text-red-500 dark:text-red-400 mb-2">Error loading epochs</h3>
            <p className="text-sm text-muted-foreground mb-4">{(error as Error).message}</p>
            <Button onClick={() => refetch()}>
              Try Again
            </Button>
          </div>
        </div>
      ) : data ? (
        <div className="space-y-4">
          {/* Stats */}
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              Showing {((page - 1) * pageSize) + 1} to{' '}
              {Math.min(page * pageSize, data.pagination.totalCount)} of{' '}
              {data.pagination.totalCount} epochs
              {hasActiveFilters && ' (filtered)'}
            </p>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">Show:</span>
                <Select value={pageSize.toString()} onValueChange={handlePageSizeChange}>
                  <SelectTrigger className="w-[70px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="5">5</SelectItem>
                    <SelectItem value="10">10</SelectItem>
                    <SelectItem value="20">20</SelectItem>
                    <SelectItem value="50">50</SelectItem>
                    <SelectItem value="100">100</SelectItem>
                  </SelectContent>
                </Select>
                <span className="text-sm text-muted-foreground">per page</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Page {page} of {data.pagination.totalPages}
              </p>
            </div>
          </div>

          {/* Table */}
          <EpochsTable epochs={data.epochs} />

          {/* Pagination */}
          {data.pagination.totalPages > 1 && (
            <div className="flex justify-center">
              <PaginationWithLinks
                page={page}
                pageSize={pageSize}
                totalCount={data.pagination.totalCount}
              />
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}
