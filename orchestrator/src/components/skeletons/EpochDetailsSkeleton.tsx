import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function EpochDetailsSkeleton() {
  return (
    <div className="flex flex-col flex-1 overflow-hidden gap-6 p-6">
      {/* Epoch Summary Cards Skeleton */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 flex-shrink-0">
        {Array.from({ length: 4 }).map((_, i) => (
          <Card key={i}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-4 rounded-full" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-8 w-24 mb-1" />
              <Skeleton className="h-3 w-32" />
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Indexes by Type Card Skeleton */}
      <Card className="flex-1 flex flex-col min-h-0">
        <CardHeader className="flex-shrink-0">
          <CardTitle className="text-lg">
            <Skeleton className="h-6 w-48" />
          </CardTitle>
          <CardDescription>
            <Skeleton className="h-4 w-64" />
          </CardDescription>
        </CardHeader>
        <CardContent className="flex-1 overflow-auto" style={{ minHeight: '0' }}>
          <div className="space-y-4">
            {/* Index Type Groups Skeleton */}
            {Array.from({ length: 3 }).map((_, groupIndex) => (
              <div key={groupIndex} className="border rounded-lg p-4">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-3 flex-wrap">
                    <Skeleton className="h-6 w-24" />
                    <Skeleton className="h-5 w-16 rounded-full" />
                    {/* Source pills */}
                    {Array.from({ length: 2 }).map((_, pillIndex) => (
                      <Skeleton key={pillIndex} className="h-5 w-20 rounded-full" />
                    ))}
                  </div>
                  <Skeleton className="h-4 w-20" />
                </div>

                {/* Individual indexes skeleton */}
                <div className="space-y-0">
                  {Array.from({ length: 2 + groupIndex }).map((_, indexIndex) => (
                    <div key={indexIndex} className="flex items-center justify-between p-3 bg-muted/30 rounded border-b border-border/50 last:border-b-0">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-3 min-w-0">
                          <Skeleton className="h-5 w-24 rounded-full flex-shrink-0" />
                          <Skeleton className="h-4 w-full max-w-md" />
                        </div>
                      </div>
                      <div className="text-right flex-shrink-0 ml-4">
                        <Skeleton className="h-4 w-16 mb-1" />
                        <Skeleton className="h-3 w-20" />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
} 