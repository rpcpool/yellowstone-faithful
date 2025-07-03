"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect } from "react";
import { GSFAIndexButton } from "../../../components/gsfa-index-button";

interface EpochIndex {
  name: string;
  type: string;
  size: number;
  status: string;
  location: string;
  source?: string;
}

interface EpochGsfaIndex {
  location: string;
  exists: boolean;
}

interface Epoch {
  id: number;
  epoch: string;
  status: "NotProcessed" | "Indexed" | "Complete";
  createdAt: string;
  updatedAt: string;
  indexes: EpochIndex[];
  gsfaIndexes: EpochGsfaIndex[];
}

async function getEpoch(id: string): Promise<Epoch | null> {
  try {
    const response = await fetch(
      `/api/epochs?epoch=${id}`,
      {
        cache: 'no-store',
      }
    );

    if (!response.ok) {
      return null;
    }

    const data = await response.json();
    if (data.epoch) {
      const epochId = parseInt(id, 10);
      
      // Extract indexes from the objects array
      const indexes: EpochIndex[] = data.epoch.objects?.map((obj: any) => {
        // Parse the name to extract type (e.g., "transactions-123" -> "transactions")
        const typeMatch = obj.name.match(/^([^-]+)-\d+$/);
        const type = typeMatch ? typeMatch[1] : obj.name;
        
        return {
          name: obj.name,
          type: type,
          size: obj.size || 0,
          status: obj.status || data.epoch.epochStatus,
          location: obj.location || '',
          source: obj.source || obj.location?.split('/')[0] || 'Unknown'
        };
      }) || [];
      
      // For now, we'll return empty gsfaIndexes as they're not in the legacy format
      const gsfaIndexes: EpochGsfaIndex[] = [];
      
      return {
        id: epochId,
        epoch: `epoch-${epochId}`,
        status: data.epoch.epochStatus || data.epoch.objects?.[0]?.status || 'NotProcessed',
        createdAt: new Date().toISOString(), // Placeholder
        updatedAt: new Date().toISOString(), // Placeholder
        indexes,
        gsfaIndexes
      };
    }
    return null;
  } catch (error) {
    console.error('Error fetching epoch:', error);
    return null;
  }
}

function getStatusColor(status: Epoch['status'] | string) {
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

export default function EpochDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = typeof params.id === 'string' ? params.id : Array.isArray(params.id) ? params.id[0] : '';

  const {
    data: epoch,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['epoch', id],
    queryFn: () => getEpoch(id),
    enabled: !!id,
  });

  // Redirect to 404 if not found (client-side)
  useEffect(() => {
    if (!isLoading && !epoch) {
      router.replace('/404');
    }
  }, [isLoading, epoch, router]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <h3 className="text-lg font-medium text-muted-foreground mb-2">Loading epoch...</h3>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <h3 className="text-lg font-medium text-red-500 dark:text-red-400 mb-2">Error loading epoch</h3>
          <p className="text-sm text-muted-foreground mb-4">{(error as Error).message}</p>
          <Button onClick={() => window.location.reload()}>Try Again</Button>
        </div>
      </div>
    );
  }

  if (!epoch) {
    // Will redirect to 404, but render nothing while waiting
    return null;
  }

  return (
    <div className="space-y-8 container mx-auto my-4">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/epochs">
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to Epochs
          </Link>
        </Button>
      </div>

      <div>
        <div className="flex items-center gap-4 mb-2">
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Epoch {epoch.id}</h1>
          <Badge className={getStatusColor(epoch.status)}>
            {epoch.status}
          </Badge>
        </div>
        <p className="text-muted-foreground">
          Detailed information about epoch {epoch.id}
        </p>
      </div>

      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Epoch Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Epoch ID
                </label>
                <p className="text-lg font-mono">{epoch.id}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Epoch String
                </label>
                <p className="text-lg font-mono">{epoch.epoch}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Status
                </label>
                <div className="mt-1">
                  <Badge className={getStatusColor(epoch.status)}>
                    {epoch.status}
                  </Badge>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Last Updated
                </label>
                <p className="text-lg">
                  {new Date(epoch.updatedAt).toLocaleDateString()}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <Card>
          <CardHeader>
            <CardTitle>Processing Details</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div>
                <h4 className="font-medium mb-2">Status Description</h4>
                <p className="text-muted-foreground">
                  {epoch.status === 'Complete' &&
                    'This epoch has been fully processed and all data is available.'}
                  {epoch.status === 'Indexed' &&
                    'This epoch has been indexed but processing is not yet complete.'}
                  {epoch.status === 'NotProcessed' &&
                    'This epoch is queued for processing but has not been started yet.'}
                </p>
              </div>

              <div>
                <h4 className="font-medium mb-2">Next Steps</h4>
                <p className="text-muted-foreground">
                  {epoch.status === 'Complete' &&
                    'No further processing required. Data is ready for use.'}
                  {epoch.status === 'Indexed' &&
                    'Processing is in progress. Check back later for completion.'}
                  {epoch.status === 'NotProcessed' &&
                    'Epoch is waiting to be processed. Processing will begin automatically.'}
                </p>
              </div>

              <div>
                <h4 className="font-medium mb-2">GSFA Index</h4>
                <p className="text-muted-foreground mb-3">
                  Download the GSFA index for this epoch. This will download the indexes required for GetSignaturesForAddress queries.
                </p>
                <GSFAIndexButton epochId={epoch.id} />
              </div>
            </div>
          </CardContent>
        </Card>

        {epoch.indexes.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Data Sources</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <p className="text-muted-foreground text-sm">
                  The following sources have data available for this epoch:
                </p>
                
                {/* Group indexes by source */}
                {Object.entries(
                  epoch.indexes.reduce((acc, index) => {
                    const source = index.source || 'Unknown';
                    if (!acc[source]) {
                      acc[source] = [];
                    }
                    acc[source].push(index);
                    return acc;
                  }, {} as Record<string, EpochIndex[]>)
                ).map(([source, sourceIndexes]) => (
                  <div key={source} className="border rounded-lg p-4">
                    <h4 className="font-medium mb-3">{source}</h4>
                    <div className="space-y-2">
                      {sourceIndexes.map((index, idx) => (
                        <div
                          key={`${source}-${index.type}-${idx}`}
                          className="flex items-center justify-between py-2 border-b last:border-0"
                        >
                          <div className="flex items-center gap-3">
                            <span className="font-mono text-sm">{index.type}</span>
                            <Badge className={`text-xs ${getStatusColor(index.status)}`}>
                              {index.status}
                            </Badge>
                          </div>
                          <div className="flex items-center gap-4 text-sm text-muted-foreground">
                            {index.size > 0 && (
                              <span>{(index.size / 1024 / 1024).toFixed(2)} MB</span>
                            )}
                            {index.location && (
                              <span className="font-mono text-xs">{index.location}</span>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}

                {epoch.gsfaIndexes.length > 0 && (
                  <div className="mt-4">
                    <h4 className="font-medium mb-2">GSFA Indexes</h4>
                    <div className="space-y-2">
                      {epoch.gsfaIndexes.map((gsfa, idx) => (
                        <div
                          key={`gsfa-${idx}`}
                          className="flex items-center justify-between py-2"
                        >
                          <span className="font-mono text-sm">{gsfa.location}</span>
                          <Badge variant={gsfa.exists ? "default" : "secondary"}>
                            {gsfa.exists ? "Available" : "Not Available"}
                          </Badge>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        )}
        </div>
      </div>
    </div>
  );
}
