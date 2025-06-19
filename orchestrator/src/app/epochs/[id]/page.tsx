import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { GSFAIndexButton } from "../../../components/gsfa-index-button";

interface Epoch {
  id: number;
  epoch: string;
  status: "NotProcessed" | "Indexed" | "Complete";
  createdAt: string;
  updatedAt: string;
}

async function getEpoch(id: string): Promise<Epoch | null> {
  const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || 'http://localhost:3000';
  
  try {
    const response = await fetch(
      `${baseUrl}/api/epochs?epoch=${id}`,
      {
        cache: 'no-store',
      }
    );

    if (!response.ok) {
      return null;
    }

    const data = await response.json();
    
    // The API returns the epoch in a nested structure
    if (data.epoch?.objects?.[0]) {
      // Extract epoch data from the API response structure
      const epochId = parseInt(id, 10);
      return {
        id: epochId,
        epoch: `epoch-${epochId}`,
        status: data.epoch.objects[0].status,
        createdAt: new Date().toISOString(), // Placeholder since API doesn't return these
        updatedAt: new Date().toISOString(), // Placeholder since API doesn't return these
      };
    }
    
    return null;
  } catch (error) {
    console.error('Error fetching epoch:', error);
    return null;
  }
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

interface EpochDetailPageProps {
  params: Promise<{
    id: string;
  }>;
}

export default async function EpochDetailPage({ params }: EpochDetailPageProps) {
  const { id } = await params;
  const epoch = await getEpoch(id);

  if (!epoch) {
    notFound();
  }

  return (
    <div className="space-y-8">
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

      <div className="grid gap-6">
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
                  Schedule a GSFA (GetSignaturesForAddress) index job for this epoch. This will create specialized indexes to optimize queries for getting signatures associated with specific addresses.
                </p>
                <GSFAIndexButton epochId={epoch.id} />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
