import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const TOTAL_EPOCHS = 792;

export function EpochGridSkeleton() {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Epoch Grid</CardTitle>
            <CardDescription>
              Loading epoch data...
            </CardDescription>
          </div>
          <div className="flex items-center gap-6 text-sm">
            <span className="font-medium text-muted-foreground">Status Legend:</span>
            <div className="flex items-center gap-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Skeleton className="w-3 h-3 rounded" />
                  <Skeleton className="h-4 w-16" />
                </div>
              ))}
            </div>
          </div>
        </div>
        
        {/* Source Filter Controls Skeleton */}
        <div className="flex flex-wrap items-center gap-4 pt-4 border-t">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-muted-foreground">Filter by source:</span>
            <Skeleton className="h-8 w-32" />
          </div>
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">or</span>
            <div className="flex flex-wrap gap-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-6 w-20 rounded-full" />
              ))}
            </div>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        <div className="grid gap-2" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(2.5rem, 1fr))' }}>
          {Array.from({ length: TOTAL_EPOCHS }).map((_, i) => (
            <Skeleton 
              key={i} 
              className="w-10 h-10 rounded animate-pulse"
              style={{ 
                animationDelay: `${(i % 50) * 20}ms`,
                animationDuration: '1.5s'
              }}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  );
} 