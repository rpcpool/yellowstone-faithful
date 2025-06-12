"use client";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { humanizeSize } from "@/lib/utils";
import { useMemo, useState } from "react";

const STATUS_COLORS: Record<string, string> = {
  NotProcessed: 'bg-muted',
  Indexed: 'bg-blue-500',
  Complete: 'bg-green-500',
};

const SOURCE_COLORS: Record<string, string> = {
  'Old Faithful': 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800',
  'HTTP': 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800',
  'S3': 'bg-orange-100 text-orange-800 border-orange-200 dark:bg-orange-900/20 dark:text-orange-300 dark:border-orange-800',
  'Local': 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800',
  'Other': 'bg-gray-100 text-gray-800 border-gray-200 dark:bg-gray-900/20 dark:text-gray-300 dark:border-gray-800',
};

type EpochData = {
  hasData: boolean;
  objects: { name: string; size: number | string | bigint; status?: string; location?: string }[];
  epochStatus?: string; // Add epoch-level status
} | null;

interface EpochGridProps {
  epochs: EpochData[];
  totalEpochs: number;
  onEpochClick: (epochIndex: number) => void;
}

// Function to determine source from object location
function determineSource(location?: string): string {
  if (!location) return 'Other';
  
  if (location.includes('old-faithful')) {
    return 'Old Faithful';
  } else if (location.includes('http')) {
    return 'HTTP';
  } else if (location.includes('s3://')) {
    return 'S3';
  } else if (location.startsWith('/data/') || location.includes('/data/cars/')) {
    return 'Local';
  }
  return 'Other';
}

// Function to get primary source for an epoch
function getEpochPrimarySource(epoch: EpochData): string {
  if (!epoch?.objects?.length) return 'Other';
  
  // Count sources in this epoch
  const sourceCounts: Record<string, number> = {};
  epoch.objects.forEach(obj => {
    const source = determineSource(obj.location);
    sourceCounts[source] = (sourceCounts[source] || 0) + 1;
  });
  
  // Return the most common source, or 'Other' if tied
  const sortedSources = Object.entries(sourceCounts).sort(([,a], [,b]) => b - a);
  return sortedSources[0]?.[0] || 'Other';
}

export function EpochGrid({ epochs, onEpochClick }: EpochGridProps) {
  const [selectedSources, setSelectedSources] = useState<string[]>([]);

  // Get all available sources from the epochs
  const availableSources = useMemo(() => {
    const sources = new Set<string>();
    epochs.forEach(epoch => {
      if (epoch?.objects) {
        epoch.objects.forEach(obj => {
          sources.add(determineSource(obj.location));
        });
      }
    });
    return Array.from(sources).sort();
  }, [epochs]);

  // Filter epochs based on selected sources
  const filteredEpochs = useMemo(() => {
    if (selectedSources.length === 0) {
      return epochs;
    }

    return epochs.map((epoch) => {
      if (!epoch) return epoch;
      
      const epochSource = getEpochPrimarySource(epoch);
      return selectedSources.includes(epochSource) ? epoch : null;
    });
  }, [epochs, selectedSources]);

  // Count filtered vs total epochs
  const filteredCount = filteredEpochs.filter(epoch => epoch !== null).length;
  const totalCount = epochs.filter(epoch => epoch !== null).length;

  const handleMouseEnter = () => () => {
    // Mouse enter logic can be added here if needed for future enhancements
  };

  const handleMouseLeave = () => {
    // Mouse leave logic can be added here if needed for future enhancements
  };

  const handleSourceFilterChange = (value: string) => {
    if (value === 'all') {
      setSelectedSources([]);
    } else {
      setSelectedSources([value]);
    }
  };

  const handleSourceToggle = (source: string) => {
    setSelectedSources(prev => 
      prev.includes(source) 
        ? prev.filter(s => s !== source)
        : [...prev, source]
    );
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Epoch Grid</CardTitle>
            <CardDescription>
              Click on any epoch to view detailed information
              {selectedSources.length > 0 && (
                <span className="ml-2 text-sm">
                  • Showing {filteredCount} of {totalCount} epochs
                </span>
              )}
            </CardDescription>
          </div>
          <div className="flex items-center gap-6 text-sm">
            <span className="font-medium text-muted-foreground">Status Legend:</span>
            <div className="flex items-center gap-4">
              {Object.entries(STATUS_COLORS).map(([status, color]) => (
                <div key={status} className="flex items-center gap-2">
                  <span className={`inline-block w-3 h-3 rounded ${color} border border-border flex-shrink-0`} />
                  <span className="text-sm font-medium capitalize">{status}</span>
                </div>
              ))}
              <div className="flex items-center gap-2">
                <span className="inline-block w-3 h-3 rounded bg-primary border border-border flex-shrink-0" />
                <span className="text-sm font-medium capitalize">Other/Unknown</span>
              </div>
            </div>
          </div>
        </div>
        
        {/* Source Filter Controls */}
        <div className="flex items-center gap-4 pt-4 border-t">
          <span className="font-medium text-muted-foreground">Filter by Source:</span>
          
          {/* Quick filter dropdown */}
          <Select value={selectedSources.length === 0 ? 'all' : selectedSources[0]} onValueChange={handleSourceFilterChange}>
            <SelectTrigger className="w-48">
              <SelectValue placeholder="All sources" />
            </SelectTrigger>
            <SelectContent className="bg-popover border border-border">
              <SelectItem value="all">All Sources</SelectItem>
              {availableSources.map(source => (
                <SelectItem key={source} value={source}>
                  {source}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Source badges for multi-select */}
          <div className="flex items-center gap-2 flex-wrap">
            {availableSources.map(source => (
              <Badge
                key={source}
                variant={selectedSources.includes(source) ? "default" : "outline"}
                className={`cursor-pointer transition-colors ${
                  selectedSources.includes(source) 
                    ? SOURCE_COLORS[source] || 'bg-accent text-accent-foreground border-border'
                    : 'hover:bg-muted'
                }`}
                onClick={() => handleSourceToggle(source)}
              >
                {source}
              </Badge>
            ))}
          </div>

          {/* Clear filters */}
          {selectedSources.length > 0 && (
            <button
              onClick={() => setSelectedSources([])}
              className="text-sm text-muted-foreground hover:text-foreground underline"
            >
              Clear filters
            </button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <TooltipProvider>
          <div className="grid gap-2" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(2.5rem, 1fr))' }}>
            {filteredEpochs.map((epoch, i) => {
              // Determine epoch status - prefer epoch-level status if available
              const getEpochStatus = (epoch: EpochData): string | undefined => {
                // If we have an epoch-level status, use that
                if (epoch?.epochStatus) {
                  return epoch.epochStatus;
                }
                
                // Fallback to analyzing individual object statuses
                if (!epoch?.objects?.length) return undefined;
                
                // Count status occurrences
                const statusCounts: Record<string, number> = {};
                epoch.objects.forEach(obj => {
                  if (obj.status) {
                    statusCounts[obj.status] = (statusCounts[obj.status] || 0) + 1;
                  }
                });
                
                // If no objects have status, return undefined
                if (Object.keys(statusCounts).length === 0) return undefined;
                
                // Priority order: Complete > Indexed > NotProcessed
                if (statusCounts['Complete'] > 0) return 'Complete';
                if (statusCounts['Indexed'] > 0) return 'Indexed';
                if (statusCounts['NotProcessed'] > 0) return 'NotProcessed';
                
                // Return the most common status as fallback
                const sortedStatuses = Object.entries(statusCounts).sort(([,a], [,b]) => b - a);
                return sortedStatuses[0]?.[0];
              };
              
              const status = getEpochStatus(epoch);
              const epochSource = epoch ? getEpochPrimarySource(epoch) : null;
              const colorClass = status ? (STATUS_COLORS[status] || 'bg-primary') : (epoch ? 'bg-muted' : 'bg-muted/50 animate-pulse');
              
              // If epoch is filtered out, show it dimmed
              const isFiltered = selectedSources.length > 0 && !epoch;
              const displayEpoch = isFiltered ? epochs[i] : epoch; // Show original epoch data for tooltip even when filtered
              const displayStatus = getEpochStatus(displayEpoch); // Get status for the display epoch
              
              return (
                <Tooltip key={i}>
                  <TooltipTrigger asChild>
                    <div
                      className={`w-10 h-10 flex items-center justify-center rounded relative cursor-pointer ${
                        isFiltered 
                          ? 'bg-muted/30 text-muted-foreground opacity-30' 
                          : colorClass
                      } ${isFiltered ? '' : (status ? 'text-white' : 'text-primary-foreground')} font-bold text-sm hover:scale-105 transition-transform duration-200`}
                      onMouseEnter={displayEpoch ? handleMouseEnter(i) : undefined}
                      onMouseMove={displayEpoch ? handleMouseEnter(i) : undefined}
                      onMouseLeave={displayEpoch ? handleMouseLeave : undefined}
                      onClick={epoch ? () => onEpochClick(i) : undefined}
                    >
                      {i}
                    </div>
                  </TooltipTrigger>
                  <TooltipContent className="max-w-xs bg-popover text-popover-foreground border border-border shadow-lg">
                    <div className="space-y-2">
                      <div className="font-bold">Epoch {i}</div>
                      <div>Status: <span className="font-mono text-sm">{displayStatus || 'unknown'}</span></div>
                      {epochSource && (
                        <div>Source: <span className="font-mono text-sm">{epochSource}</span></div>
                      )}
                      {isFiltered && (
                        <div className="text-yellow-500 dark:text-yellow-400 text-xs">Filtered out</div>
                      )}
                      {displayEpoch && displayEpoch.objects.length === 0 ? (
                        <div className="text-muted-foreground">No objects</div>
                      ) : (
                        <div className="space-y-1">
                          <div className="text-sm font-medium">Objects:</div>
                          <ul className="space-y-1 text-xs">
                            {displayEpoch?.objects.slice(0, 5).map((obj, j) => (
                              <li key={j} className="truncate">
                                <span className="font-mono">{obj.name}</span> — {humanizeSize(obj.size || 0)}
                                {obj.location && (
                                  <span className="text-muted-foreground ml-1">
                                    ({determineSource(obj.location)})
                                  </span>
                                )}
                              </li>
                            ))}
                            {displayEpoch?.objects && displayEpoch.objects.length > 5 && (
                              <li className="text-muted-foreground">
                                ... and {displayEpoch.objects.length - 5} more
                              </li>
                            )}
                          </ul>
                        </div>
                      )}
                    </div>
                  </TooltipContent>
                </Tooltip>
              );
            })}
          </div>
        </TooltipProvider>
      </CardContent>
    </Card>
  );
} 