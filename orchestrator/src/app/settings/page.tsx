"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useState, useEffect } from "react";
import { useMutation } from "@tanstack/react-query";

interface Source {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
}

export default function SettingsPage() {
  const [isReindexing, setIsReindexing] = useState(false);
  const [selectedSource, setSelectedSource] = useState<string>("all");
  const [sources, setSources] = useState<Source[]>([]);
  const [isLoadingSources, setIsLoadingSources] = useState(true);
  const [lastReindexResult, setLastReindexResult] = useState<{
    success: boolean;
    message: string;
    timestamp: Date;
  } | null>(null);

  // Fetch sources on component mount
  useEffect(() => {
    const fetchSources = async () => {
      try {
        const response = await fetch('/api/sources?enabled=true&pageSize=100');
        if (response.ok) {
          const data = await response.json();
          setSources(data.sources || []);
        } else {
          console.error('Failed to fetch sources');
        }
      } catch (error) {
        console.error('Error fetching sources:', error);
      } finally {
        setIsLoadingSources(false);
      }
    };

    fetchSources();
  }, []);

  const mutation = useMutation({
    mutationFn: async () => {
      const response = await fetch('/api/jobs/schedule', { 
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          jobType: 'refreshAllEpochs',
          params: {
            ...(selectedSource !== 'all' && { sourceName: selectedSource }),
            batchSize: 100 // Process 100 epochs at a time
          }
        })
      });
      
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.details || error.error || 'Failed to schedule job');
      }
      
      return response.json();
    },
    onSuccess: (data) => {
      setLastReindexResult({ success: data.success, message: data.message, timestamp: new Date() });
    },
    onError: (error: unknown) => {
      setLastReindexResult({
        success: false,
        message: error instanceof Error ? error.message : 'Failed to trigger re-indexing',
        timestamp: new Date(),
      });
    },
    onSettled: () => {
      setIsReindexing(false);
    },
  });

  const handleReindex = () => {
    setIsReindexing(true);
    setLastReindexResult(null);
    mutation.mutate();
  };

  return (
    <div className="container mx-auto p-6 max-w-4xl">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground">Settings</h1>
        <p className="text-muted-foreground mt-2">
          Manage system settings and maintenance operations
        </p>
      </div>

      <div className="grid gap-6">
        {/* Re-index Remote Files Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
              Re-index Data Sources
            </CardTitle>
            <CardDescription>
              Schedule a background job to re-index data from selected sources. This will check the availability 
              of data files and update the database with the current status. You can choose to re-index all 
              sources or select a specific source to refresh.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center">
              <Select value={selectedSource} onValueChange={setSelectedSource} disabled={isLoadingSources || sources.length === 0}>
                <SelectTrigger className="w-full sm:w-[250px]">
                  <SelectValue placeholder={
                    isLoadingSources ? "Loading sources..." : 
                    sources.length === 0 ? "No sources configured" : 
                    "Select source"
                  } />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Sources</SelectItem>
                  {sources.length === 0 && !isLoadingSources && (
                    <SelectItem value="" disabled>No sources available</SelectItem>
                  )}
                  {sources.map((source) => (
                    <SelectItem key={source.id} value={source.name}>
                      {source.name} ({source.type})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                onClick={handleReindex}
                disabled={isReindexing || isLoadingSources || sources.length === 0}
                className="w-full sm:w-auto"
              >
                {isReindexing ? (
                  <>
                    <svg
                      className="animate-spin -ml-1 mr-2 h-4 w-4"
                      xmlns="http://www.w3.org/2000/svg"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      ></circle>
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                                         Scheduling...
                   </>
                 ) : (
                   selectedSource === 'all' ? 'Schedule Re-indexing (All Sources)' : `Schedule Re-indexing (${selectedSource})`
                 )}
              </Button>
              
                             {isReindexing && (
                 <div className="text-sm text-muted-foreground">
                   Scheduling background job...
                 </div>
               )}
            </div>

            {lastReindexResult && (
              <div
                className={`p-4 rounded-lg border ${
                  lastReindexResult.success
                                    ? 'bg-green-50 border-green-200 text-green-800 dark:bg-green-900/20 dark:border-green-800 dark:text-green-300'
                : 'bg-red-50 border-red-200 text-red-800 dark:bg-red-900/20 dark:border-red-800 dark:text-red-300'
                }`}
              >
                <div className="flex items-start gap-2">
                  <svg
                    className={`w-5 h-5 mt-0.5 flex-shrink-0 ${
                      lastReindexResult.success ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
                    }`}
                    fill="currentColor"
                    viewBox="0 0 20 20"
                  >
                    {lastReindexResult.success ? (
                      <path
                        fillRule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                        clipRule="evenodd"
                      />
                    ) : (
                      <path
                        fillRule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                        clipRule="evenodd"
                      />
                    )}
                  </svg>
                  <div className="flex-1">
                    <p className="font-medium">
                      {lastReindexResult.success ? 'Success' : 'Error'}
                    </p>
                    <p className="text-sm mt-1">{lastReindexResult.message}</p>
                    <p className="text-xs mt-2 opacity-75">
                      {lastReindexResult.timestamp.toLocaleString()}
                    </p>
                  </div>
                </div>
              </div>
            )}

                         <div className="text-sm text-muted-foreground space-y-1">
               <p><strong>What this does:</strong></p>
               <ul className="list-disc list-inside space-y-1 ml-2">
                 <li>Schedules a background job via Faktory worker system</li>
                 <li>Refreshes data availability status for selected source(s)</li>
                 <li>Updates database records with current file status</li>
                 <li>Supports filtering by specific data source (S3, HTTP, Local)</li>
                 <li>Processes all epochs for the selected source(s)</li>
               </ul>
               <p className="mt-2"><strong>Note:</strong> The job runs in the background and may take several minutes to complete. Check the worker logs for progress updates.</p>
             </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
