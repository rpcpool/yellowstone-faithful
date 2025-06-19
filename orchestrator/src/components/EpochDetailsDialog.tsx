"use client";

import { GSFAIndexButton } from "@/components/gsfa-index-button";
import { RefreshEpochButton } from "@/components/RefreshEpochButton";
import { EpochDetailsSkeleton } from "@/components/skeletons/EpochDetailsSkeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { humanizeSize } from "@/lib/utils";


const SOURCE_STYLES: Record<string, string> = {
  'Old Faithful': 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800',
  'HTTP': 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800',
  'S3': 'bg-orange-100 text-orange-800 border-orange-200 dark:bg-orange-900/20 dark:text-orange-300 dark:border-orange-800',
  'Local': 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800',
  'Unknown': 'bg-muted text-muted-foreground border-border',
};

type EpochDetails = {
  epoch: {
    id: number;
    epoch: string;
    status: string;
    createdAt: string;
    updatedAt: string;
  };
  indexes: Array<{
    id: number;
    epoch: string;
    type: string;
    size: string;
    status: string;
    location: string;
    createdAt: string;
    updatedAt: string;
  }>;
  gsfa: {
    id: number;
    epoch: string;
    exists: boolean;
    location: string;
    createdAt: string;
    updatedAt: string;
  } | null;
  stats: {
    totalIndexes: number;
    totalSize: number;
    statusCounts: Record<string, number>;
    typeCounts: Record<string, number>;
  };
};

interface EpochDetailsDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  selectedEpoch: number | null;
  epochDetails: EpochDetails | null;
  isLoadingDetails: boolean;
  onRetry: () => void;
}

export function EpochDetailsDialog({
  isOpen,
  onOpenChange,
  selectedEpoch,
  epochDetails,
  isLoadingDetails,
  onRetry
}: EpochDetailsDialogProps) {
  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className="!max-w-[50vw] w-full max-h-[85vh] overflow-hidden flex flex-col text-foreground">
        <DialogHeader className="flex-shrink-0">
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1 min-w-0">
              <DialogTitle>Epoch {selectedEpoch} Details</DialogTitle>
              <DialogDescription>
                Detailed information about indexes and statistics for this epoch
              </DialogDescription>
            </div>
            {selectedEpoch !== null && (
              <div className="flex-shrink-0">
                <RefreshEpochButton epochId={selectedEpoch} />
              </div>
            )}
          </div>
        </DialogHeader>

        {isLoadingDetails ? (
          <EpochDetailsSkeleton />
        ) : epochDetails ? (
          <div className="flex flex-col flex-1 overflow-hidden gap-6 p-6">
            {/* Epoch Summary - Fixed */}
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 flex-shrink-0">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Status</CardTitle>
                  <div className={`h-4 w-4 rounded-full ${
                    epochDetails.epoch.status === 'Complete' 
                      ? 'bg-green-500' 
                      : epochDetails.epoch.status === 'Indexed'
                      ? 'bg-blue-500'
                      : 'bg-muted'
                  }`} />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{epochDetails.epoch.status}</div>
                  <p className="text-xs text-muted-foreground">
                    Updated {new Date(epochDetails.epoch.updatedAt).toLocaleDateString()}
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total Indexes</CardTitle>
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    className="h-4 w-4 text-muted-foreground"
                  >
                    <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
                    <circle cx="9" cy="7" r="4" />
                    <path d="m22 21-3-3" />
                  </svg>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{epochDetails.stats.totalIndexes}</div>
                  <p className="text-xs text-muted-foreground">
                    Across all sources
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total Size</CardTitle>
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    className="h-4 w-4 text-muted-foreground"
                  >
                    <rect width="20" height="14" x="2" y="5" rx="2" />
                    <path d="M2 10h20" />
                  </svg>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{humanizeSize(epochDetails.stats.totalSize)}</div>
                  <p className="text-xs text-muted-foreground">
                    Combined storage
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">GSFA Index</CardTitle>
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    className="h-4 w-4 text-muted-foreground"
                  >
                    <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
                  </svg>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {epochDetails.gsfa && epochDetails.gsfa.exists ? "Available" : "Missing"}
                  </div>
                  <p className="text-xs text-muted-foreground mb-3">
                    {epochDetails.gsfa && epochDetails.gsfa.exists 
                      ? `Updated ${new Date(epochDetails.gsfa.updatedAt).toLocaleDateString()}`
                      : "Not generated yet"
                    }
                  </p>
                  {(!epochDetails.gsfa || !epochDetails.gsfa.exists) && selectedEpoch !== null && (
                    <div className="mt-3">
                      <GSFAIndexButton epochId={selectedEpoch} />
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>

            {/* Indexes by Type - Scrollable */}
            <Card className="flex-1 flex flex-col min-h-0">
              <CardHeader className="flex-shrink-0">
                <CardTitle className="text-lg">Indexes by Type ({epochDetails.indexes.length} total)</CardTitle>
                <CardDescription>
                  All indexes for this epoch, grouped by type.
                </CardDescription>
              </CardHeader>
              <CardContent className="flex-1 overflow-auto" style={{ minHeight: '0' }}>
                <div className="space-y-4">
                  {(() => {
                    // Group indexes by type and collect sources for each type
                    const indexesByType = epochDetails.indexes.reduce((acc, index) => {
                      let source = 'Unknown';
                      if (index.location.includes('old-faithful')) {
                        source = 'Old Faithful';
                      } else if (index.location.includes('http')) {
                        source = 'HTTP';
                      } else if (index.location.includes('s3://')) {
                        source = 'S3';
                      } else if (index.location.includes('/')) {
                        // Try to extract a more meaningful source from the path
                        const parts = index.location.split('/');
                        source = parts[0] || 'Local';
                      }

                      if (!acc[index.type]) {
                        acc[index.type] = {
                          indexes: [],
                          sources: new Set(),
                          totalSize: 0
                        };
                      }
                      acc[index.type].indexes.push(index);
                      acc[index.type].sources.add(source);
                      acc[index.type].totalSize += Number(index.size);
                      return acc;
                    }, {} as Record<string, { indexes: typeof epochDetails.indexes, sources: Set<string>, totalSize: number }>);

                    return Object.entries(indexesByType).map(([type, data]) => (
                      <div key={type} className="border rounded-lg p-4">
                        <div className="flex items-center justify-between mb-3">
                          <div className="flex items-center gap-3 flex-wrap">
                            <h4 className="font-semibold text-base">{type}</h4>
                            <Badge variant="secondary" className="text-xs">
                              {data.indexes.length} {data.indexes.length === 1 ? 'source' : 'sources'}
                            </Badge>
                            {/* Source pills next to index name */}
                            {Array.from(data.sources).map((source) => (
                              <Badge key={source} variant="outline" className={`text-xs ${SOURCE_STYLES[source] || SOURCE_STYLES['Unknown']}`}>
                                {source}
                              </Badge>
                            ))}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            Total: {humanizeSize(data.totalSize)}
                          </div>
                        </div>

                        {/* Individual indexes */}
                        <div className="space-y-0">
                          {data.indexes.map((index) => {
                            let indexSource = 'Unknown';
                            if (index.location.includes('old-faithful')) {
                              indexSource = 'Old Faithful';
                            } else if (index.location.includes('http')) {
                              indexSource = 'HTTP';
                            } else if (index.location.includes('s3://')) {
                              indexSource = 'S3';
                            } else if (index.location.includes('/')) {
                              const parts = index.location.split('/');
                              indexSource = parts[0] || 'Local';
                            }

                            return (
                              <div key={index.id} className="flex items-center justify-between p-3 bg-muted/30 rounded border-b border-border/50 last:border-b-0">
                                <div className="flex-1 min-w-0">
                                  <div className="flex items-center gap-3 min-w-0">
                                    <Badge variant="outline" className={`text-xs w-24 justify-center flex-shrink-0 ${SOURCE_STYLES[indexSource] || SOURCE_STYLES['Unknown']}`}>
                                      {indexSource}
                                    </Badge>
                                    <p className="text-sm text-muted-foreground truncate min-w-0 flex-1">
                                      {index.location}
                                    </p>
                                  </div>
                                </div>
                                <div className="text-right flex-shrink-0 ml-4">
                                  <p className="text-sm font-medium">
                                    {humanizeSize(Number(index.size))}
                                  </p>
                                  <p className="text-xs text-muted-foreground">
                                    {new Date(index.updatedAt).toLocaleDateString()}
                                  </p>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    ));
                  })()}
                </div>
              </CardContent>
            </Card>
          </div>
        ) : (
          <div className="text-center py-8 flex-1 flex items-center justify-center">
            <div>
              <p className="text-muted-foreground">Failed to load epoch details. Please try again.</p>
              <Button onClick={onRetry} className="mt-4">
                Retry
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
} 