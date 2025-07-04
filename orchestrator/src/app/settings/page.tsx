"use client";

import { AddAllEpochsCard } from "@/components/settings/AddAllEpochsCard";
import { AddEpochRangeCard } from "@/components/settings/AddEpochRangeCard";
import { AddSingleEpochCard } from "@/components/settings/AddSingleEpochCard";
import { EpochsResultAlert } from "@/components/settings/EpochsResultAlert";
import { EpochsStatusOverviewCard } from "@/components/settings/EpochsStatusOverviewCard";
import { GeneralSettingsCard } from "@/components/settings/GeneralSettingsCard";
import { JobsCard } from "@/components/settings/JobsCard";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { deleteJob, fetchJobs } from "@/lib/api";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";

interface Source {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
}

interface EpochData {
  id: number;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export default function SettingsPage() {
  const queryClient = useQueryClient();
  const searchParams = useSearchParams();
  
  // Get initial values from URL params
  const initialTab = searchParams.get("tab") || "general";
  const initialSource = searchParams.get("source") || "all";
  
  // General tab state
  const [isReindexing, setIsReindexing] = useState(false);
  const [selectedSource, setSelectedSource] = useState<string>(initialSource);
  const [sources, setSources] = useState<Source[]>([]);
  const [isLoadingSources, setIsLoadingSources] = useState(true);
  const [lastReindexResult, setLastReindexResult] = useState<{
    success: boolean;
    message: string;
    timestamp: Date;
  } | null>(null);

  // Epochs tab state
  const [singleEpoch, setSingleEpoch] = useState("");
  const [rangeStart, setRangeStart] = useState("");
  const [rangeEnd, setRangeEnd] = useState("");
  const [currentEpoch, setCurrentEpoch] = useState<number | null>(null);
  const [addResult, setAddResult] = useState<{
    success: boolean;
    message: string;
  } | null>(null);

  // Jobs tab state
  const [epochIdFilter, setEpochIdFilter] = useState<string>("");
  const [jobTypeFilter, setJobTypeFilter] = useState<string>("");
  const [currentPage, setCurrentPage] = useState(1);

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

  // Fetch current epoch
  useEffect(() => {
    const fetchCurrentEpoch = async () => {
      try {
        const response = await fetch("/api/epochs/current");
        if (response.ok) {
          const data = await response.json();
          setCurrentEpoch(data.epoch);
        }
      } catch (error) {
        console.error("Failed to fetch current epoch:", error);
      }
    };
    fetchCurrentEpoch();
  }, []);

  // Re-index mutation
  const reindexMutation = useMutation({
    mutationFn: async () => {
      // Find the source ID if a specific source is selected
      let sourceId = undefined;
      if (selectedSource !== 'all') {
        const source = sources.find(s => s.name === selectedSource);
        if (source) {
          sourceId = source.id;
        }
      }
      
      const response = await fetch('/api/jobs/schedule', { 
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          jobType: 'refreshAllEpochs',
          params: {
            ...(sourceId && { sourceId }),
            batchSize: 100
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

  // Fetch existing epochs
  const { data: epochsData } = useQuery({
    queryKey: ["epochs-overview"],
    queryFn: async () => {
      const response = await fetch("/api/epochs?page=1&pageSize=100");
      if (!response.ok) throw new Error("Failed to fetch epochs");
      return response.json();
    },
  });

  // Add epochs mutation
  const addEpochsMutation = useMutation({
    mutationFn: async (params: { epochs?: number[]; startEpoch?: number; endEpoch?: number }) => {
      const response = await fetch("/api/epochs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(params),
      });
      
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || "Failed to add epochs");
      }
      
      return response.json();
    },
    onSuccess: (data) => {
      setAddResult({ success: true, message: data.message });
      setSingleEpoch("");
      setRangeStart("");
      setRangeEnd("");
      queryClient.invalidateQueries({ queryKey: ["epochs-overview"] });
    },
    onError: (error: Error) => {
      setAddResult({ success: false, message: error.message });
    },
  });

  // Fetch jobs
  const { data: jobsData, isLoading: isLoadingJobs, error: jobsError } = useQuery({
    queryKey: ["jobs", epochIdFilter, jobTypeFilter, currentPage],
    queryFn: () => fetchJobs({
      page: currentPage,
      pageSize: 20,
      epochId: epochIdFilter ? parseInt(epochIdFilter, 10) : undefined,
      jobType: jobTypeFilter && jobTypeFilter.trim() ? jobTypeFilter : undefined,
    }),
  });

  // Delete job mutation
  const deleteJobMutation = useMutation({
    mutationFn: deleteJob,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
    },
  });

  const handleReindex = () => {
    setIsReindexing(true);
    setLastReindexResult(null);
    reindexMutation.mutate();
  };

  const handleAddSingleEpoch = () => {
    const epochNum = parseInt(singleEpoch, 10);
    if (isNaN(epochNum) || epochNum < 0) {
      setAddResult({ success: false, message: "Invalid epoch number" });
      return;
    }
    addEpochsMutation.mutate({ epochs: [epochNum] });
  };

  const handleAddRange = () => {
    const start = parseInt(rangeStart, 10);
    const end = parseInt(rangeEnd, 10);
    
    if (isNaN(start) || isNaN(end) || start < 0 || end < 0) {
      setAddResult({ success: false, message: "Invalid epoch range" });
      return;
    }
    
    if (start > end) {
      setAddResult({ success: false, message: "Start epoch must be less than or equal to end epoch" });
      return;
    }
    
    addEpochsMutation.mutate({ startEpoch: start, endEpoch: end });
  };

  const handleAddAllToDate = () => {
    if (currentEpoch === null) {
      setAddResult({ success: false, message: "Current epoch not available" });
      return;
    }
    addEpochsMutation.mutate({ startEpoch: 0, endEpoch: currentEpoch });
  };


  const existingEpochs = epochsData?.epochs || [];
  const epochStats = existingEpochs.reduce((acc: Record<string, number>, epoch: EpochData) => {
    acc[epoch.status] = (acc[epoch.status] || 0) + 1;
    return acc;
  }, {});

  const jobs = jobsData?.jobs || [];
  const pagination = jobsData?.pagination;

  return (
    <div className="container mx-auto p-6 max-w-4xl">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground">Settings</h1>
        <p className="text-muted-foreground mt-2">
          Manage system settings and maintenance operations
        </p>
      </div>

      <Tabs defaultValue={initialTab} className="space-y-4">
        <TabsList className="grid w-full grid-cols-3">
          <TabsTrigger value="general">General</TabsTrigger>
          <TabsTrigger value="epochs">Epochs</TabsTrigger>
          <TabsTrigger value="jobs">Jobs</TabsTrigger>
        </TabsList>

        {/* General Tab */}
        <TabsContent value="general" className="space-y-4">
          <GeneralSettingsCard
            selectedSource={selectedSource}
            setSelectedSource={setSelectedSource}
            isLoadingSources={isLoadingSources}
            sources={sources}
            isReindexing={isReindexing}
            handleReindex={handleReindex}
            lastReindexResult={lastReindexResult}
          />
        </TabsContent>

        {/* Epochs Tab */}
        <TabsContent value="epochs" className="space-y-6">
          <EpochsResultAlert addResult={addResult} />
          <EpochsStatusOverviewCard
            epochsData={epochsData}
            epochStats={epochStats}
            currentEpoch={currentEpoch}
          />
          <AddSingleEpochCard
            singleEpoch={singleEpoch}
            setSingleEpoch={setSingleEpoch}
            handleAddSingleEpoch={handleAddSingleEpoch}
            isPending={addEpochsMutation.isPending}
          />
          <AddEpochRangeCard
            rangeStart={rangeStart}
            setRangeStart={setRangeStart}
            rangeEnd={rangeEnd}
            setRangeEnd={setRangeEnd}
            handleAddRange={handleAddRange}
            isPending={addEpochsMutation.isPending}
          />
          <AddAllEpochsCard
            currentEpoch={currentEpoch}
            handleAddAllToDate={handleAddAllToDate}
            isPending={addEpochsMutation.isPending}
          />
        </TabsContent>

        {/* Jobs Tab */}
        <TabsContent value="jobs" className="space-y-4">
          <JobsCard
            epochIdFilter={epochIdFilter}
            setEpochIdFilter={setEpochIdFilter}
            jobTypeFilter={jobTypeFilter}
            setJobTypeFilter={setJobTypeFilter}
            currentPage={currentPage}
            setCurrentPage={setCurrentPage}
            jobs={jobs}
            isLoadingJobs={isLoadingJobs}
            jobsError={jobsError}
            pagination={pagination}
            deleteJobMutation={deleteJobMutation}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}