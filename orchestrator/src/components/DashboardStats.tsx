"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { humanizeSize } from "@/lib/utils";
import {
  Activity,
  BarChart3,
  Clock,
  Database,
  FileText,
  Globe,
  HardDrive,
  Layers,
  TrendingUp
} from "lucide-react";

const TOTAL_EPOCHS = 792;

type StatsData = {
  totalSize: string;
  totalIndexes: number;
  gsfaEpochCount: number; // Count of epochs with GSFA indexes
  statusDistribution: Record<string, number>;
  typeDistribution: Record<string, number>;
  sourceDistribution: Record<string, number>;
  typeSizeDistribution?: Record<string, number>; // Size in bytes for each type
};

interface DashboardStatsProps {
  stats: StatsData | null;
}

export function DashboardStats({ stats }: DashboardStatsProps) {
  // Get total index size from stats API (more accurate than calculating from epoch data)
  const totalIndexSize = stats ? parseInt(stats.totalSize, 10) : 0;

  return (
    <div className="space-y-4">
      {/* Header Section */}
      <div className="space-y-2">
        <h1 className="text-3xl font-bold tracking-tight">Epochs Dashboard</h1>
        <p className="text-muted-foreground">
          Monitoring {TOTAL_EPOCHS} epochs with real-time status updates
        </p>
      </div>

      {/* Key Metrics Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Epochs</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="text-center">
            <div className="text-2xl font-bold">{TOTAL_EPOCHS.toLocaleString()}</div>
            <p className="text-xs text-muted-foreground">
              Being monitored
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">GSFA Epochs</CardTitle>
            <FileText className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="text-center">
            <div className="text-2xl font-bold">
              {stats ? stats.gsfaEpochCount.toLocaleString() : "Loading..."}
            </div>
            <p className="text-xs text-muted-foreground">
              Epochs with GSFA indexes
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Data Sources</CardTitle>
            <Layers className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="text-center">
            <div className="text-2xl font-bold">
              {stats ? Object.keys(stats.sourceDistribution).length : "Loading..."}
            </div>
            <p className="text-xs text-muted-foreground">
              Active sources
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Indexes</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="text-center">
            <div className="text-2xl font-bold">
              {stats ? stats.totalIndexes.toLocaleString() : "Loading..."}
            </div>
            <p className="text-xs text-muted-foreground">
              Individual indexes
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Storage</CardTitle>
            <HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="text-center">
            <div className="text-2xl font-bold">
              {stats ? humanizeSize(totalIndexSize) : "Loading..."}
            </div>
            <p className="text-xs text-muted-foreground">
              Across all sources
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Stats Section */}
      <div className="grid gap-4 lg:grid-cols-3">
        {/* Status Distribution */}
        <Card className="col-span-1">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Activity className="h-5 w-5" />
              Epoch Status Distribution
            </CardTitle>
            <CardDescription>
              Current status of all epochs
            </CardDescription>
          </CardHeader>
          <CardContent>
            {stats ? (
              <div className="space-y-3">
                {Object.entries(stats.statusDistribution).map(([status, count]) => {
                  const percentage = ((count / TOTAL_EPOCHS) * 100).toFixed(1);
                  const getStatusIcon = (status: string) => {
                    switch (status.toLowerCase()) {
                      case 'complete':
                        return <TrendingUp className="h-4 w-4 text-green-500" />;
                      case 'indexed':
                        return <Database className="h-4 w-4 text-blue-500" />;
                      case 'notprocessed':
                        return <Clock className="h-4 w-4 text-orange-500" />;
                      default:
                        return <BarChart3 className="h-4 w-4 text-gray-500" />;
                    }
                  };
                  
                  return (
                    <div key={status} className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        {getStatusIcon(status)}
                        <div className="flex flex-col">
                          <span className="text-sm font-medium">
                            {status.replace('_', ' ')}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {percentage}% of total
                          </span>
                        </div>
                      </div>
                      <div className="text-right">
                        <div className="text-lg font-bold">{count.toLocaleString()}</div>
                        <div className="text-xs text-muted-foreground">epochs</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="flex items-center justify-center h-32">
                <div className="text-sm text-muted-foreground animate-pulse">
                  Loading data...
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Source Distribution */}
        <Card className="col-span-1">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe className="h-5 w-5" />
              Data Sources
            </CardTitle>
            <CardDescription>
              Distribution by data source
            </CardDescription>
          </CardHeader>
          <CardContent>
            {stats ? (
              <div className="space-y-3">
                {Object.entries(stats.sourceDistribution).map(([source, count]) => {
                  const percentage = ((count / stats.totalIndexes) * 100).toFixed(1);
                  const getSourceIcon = (source: string) => {
                    switch (source.toLowerCase()) {
                      case 'old faithful':
                        return <Clock className="h-4 w-4 text-purple-500" />;
                      case 'http':
                        return <Globe className="h-4 w-4 text-green-500" />;
                      case 's3':
                        return <HardDrive className="h-4 w-4 text-orange-500" />;
                      default:
                        return <Database className="h-4 w-4 text-blue-500" />;
                    }
                  };
                  
                  return (
                    <div key={source} className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        {getSourceIcon(source)}
                        <div className="flex flex-col">
                          <span className="text-sm font-medium">{source}</span>
                          <span className="text-xs text-muted-foreground">
                            {percentage}% of indexes
                          </span>
                        </div>
                      </div>
                      <div className="text-right">
                        <div className="text-lg font-bold">{count.toLocaleString()}</div>
                        <div className="text-xs text-muted-foreground">indexes</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="flex items-center justify-center h-32">
                <div className="text-sm text-muted-foreground animate-pulse">
                  Loading data...
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Storage Sizes by Type */}
        <Card className="col-span-1">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <HardDrive className="h-5 w-5" />
              Storage Sizes
            </CardTitle>
            <CardDescription>
              Total storage by index type
            </CardDescription>
          </CardHeader>
          <CardContent>
            {stats ? (
              <div className="space-y-3">
                {stats.typeSizeDistribution ? 
                  // Show storage sizes if available
                  Object.entries(stats.typeSizeDistribution).map(([type, sizeBytes]) => {
                    const percentage = ((sizeBytes / totalIndexSize) * 100).toFixed(1);
                    const displayType = type.replace(/([A-Z])/g, ' $1').trim();
                    const getTypeIcon = (type: string) => {
                      if (type.toLowerCase().includes('gsfa')) {
                        return <FileText className="h-4 w-4 text-blue-500" />;
                      } else if (type.toLowerCase().includes('cdf')) {
                        return <Database className="h-4 w-4 text-green-500" />;
                      } else {
                        return <Layers className="h-4 w-4 text-purple-500" />;
                      }
                    };
                    
                    return (
                      <div key={type} className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {getTypeIcon(type)}
                          <div className="flex flex-col">
                            <span className="text-sm font-medium">{displayType}</span>
                            <span className="text-xs text-muted-foreground">
                              {percentage}% of total storage
                            </span>
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="text-lg font-bold">{humanizeSize(sizeBytes)}</div>
                          <div className="text-xs text-muted-foreground">storage</div>
                        </div>
                      </div>
                    );
                  })
                  :
                  // Fallback to count-based display if size data not available
                  Object.entries(stats.typeDistribution).map(([type, count]) => {
                    const percentage = ((count / stats.totalIndexes) * 100).toFixed(1);
                    const displayType = type.replace(/([A-Z])/g, ' $1').trim();
                    const getTypeIcon = (type: string) => {
                      if (type.toLowerCase().includes('gsfa')) {
                        return <FileText className="h-4 w-4 text-blue-500" />;
                      } else if (type.toLowerCase().includes('cdf')) {
                        return <Database className="h-4 w-4 text-green-500" />;
                      } else {
                        return <Layers className="h-4 w-4 text-purple-500" />;
                      }
                    };
                    
                    return (
                      <div key={type} className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {getTypeIcon(type)}
                          <div className="flex flex-col">
                            <span className="text-sm font-medium">{displayType}</span>
                            <span className="text-xs text-muted-foreground">
                              {percentage}% of indexes
                            </span>
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="text-lg font-bold">{count.toLocaleString()}</div>
                          <div className="text-xs text-muted-foreground">indexes</div>
                        </div>
                      </div>
                    );
                  })
                }
              </div>
            ) : (
              <div className="flex items-center justify-center h-32">
                <div className="text-sm text-muted-foreground animate-pulse">
                  Loading data...
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
} 