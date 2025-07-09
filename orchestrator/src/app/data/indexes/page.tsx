"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { fetchIndexes } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

export default function ManageIndexesPage() {
  const [sourceFilter, setSourceFilter] = useState<string>("");
  const [typeFilter, setTypeFilter] = useState<string>("");
  const [search, setSearch] = useState<string>("");
  const [page, setPage] = useState(1);
  const [sortBy, setSortBy] = useState<string>("createdAt");
  const [sortOrder, setSortOrder] = useState<string>("desc");

  const { data, isLoading, error } = useQuery({
    queryKey: ["indexes", sourceFilter, typeFilter, search, page, sortBy, sortOrder],
    queryFn: () =>
      fetchIndexes({
        page,
        pageSize: 20,
        source: sourceFilter || undefined,
        type: typeFilter || undefined,
        search: search || undefined,
        sortBy: sortBy || undefined,
        sortOrder: sortOrder || undefined,
      }),
  });

  const indexes = data?.indexes || [];
  const pagination = data?.pagination;
  const availableSources = data?.availableSources || [];
  const availableTypes = data?.availableTypes || [];

  return (
    <div className="container mx-auto p-6 max-w-6xl space-y-6">
      <div className="mb-4">
        <h1 className="text-3xl font-bold text-foreground">Manage Indexes</h1>
        <p className="text-muted-foreground mt-2">View index files by source and type</p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Indexes</CardTitle>
          <CardDescription>All discovered indexes</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col sm:flex-row gap-4 mb-4">
            <div className="flex-1">
              <Label htmlFor="search">Search</Label>
              <Input
                id="search"
                placeholder="Search by epoch or ID"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(1);
                }}
              />
            </div>
            <div className="flex-1">
              <Label htmlFor="source">Source</Label>
              <Select
                value={sourceFilter || "all"}
                onValueChange={(value) => {
                  setSourceFilter(value === "all" ? "" : value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="All sources" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All sources</SelectItem>
                  {availableSources.map((src) => (
                    <SelectItem key={src} value={src}>
                      {src}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex-1">
              <Label htmlFor="type">Type</Label>
              <Select
                value={typeFilter || "all"}
                onValueChange={(value) => {
                  setTypeFilter(value === "all" ? "" : value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="All types" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All types</SelectItem>
                  {availableTypes.map((t) => (
                    <SelectItem key={t} value={t}>
                      {t}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex-1">
              <Label htmlFor="sortBy">Sort by</Label>
              <Select
                value={sortBy}
                onValueChange={(value) => {
                  setSortBy(value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="createdAt">Created Date</SelectItem>
                  <SelectItem value="updatedAt">Updated Date</SelectItem>
                  <SelectItem value="epoch">Epoch</SelectItem>
                  <SelectItem value="type">Type</SelectItem>
                  <SelectItem value="source">Source</SelectItem>
                  <SelectItem value="status">Status</SelectItem>
                  <SelectItem value="size">Size</SelectItem>
                  <SelectItem value="id">ID</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex-1">
              <Label htmlFor="sortOrder">Order</Label>
              <Select
                value={sortOrder}
                onValueChange={(value) => {
                  setSortOrder(value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="desc">Descending</SelectItem>
                  <SelectItem value="asc">Ascending</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-end">
              <Button
                variant="outline"
                onClick={() => {
                  setSourceFilter("");
                  setTypeFilter("");
                  setSearch("");
                  setSortBy("createdAt");
                  setSortOrder("desc");
                  setPage(1);
                }}
              >
                Clear Filters
              </Button>
            </div>
          </div>

          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : error ? (
            <p className="text-sm text-red-500">{(error as Error).message}</p>
          ) : indexes.length === 0 ? (
            <p className="text-sm text-muted-foreground">No indexes found.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Epoch</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead className="text-right">Size</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {indexes.map((idx) => (
                  <TableRow key={idx.id}>
                    <TableCell className="font-mono text-xs">{idx.id}</TableCell>
                    <TableCell>{idx.epoch}</TableCell>
                    <TableCell>{idx.type}</TableCell>
                    <TableCell>{idx.source}</TableCell>
                    <TableCell className="text-right">{Number(idx.size).toLocaleString()}</TableCell>
                    <TableCell>{idx.status}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}

          {pagination && pagination.totalPages > 1 && (
            <div className="flex items-center justify-between mt-4">
              <div className="text-sm text-muted-foreground">
                Showing {(pagination.page - 1) * pagination.pageSize + 1} to {Math.min(pagination.page * pagination.pageSize, pagination.totalCount)} of {pagination.totalCount} indexes
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(page - 1)}
                  disabled={page <= 1}
                >
                  Previous
                </Button>
                <span className="text-sm">Page {pagination.page} of {pagination.totalPages}</span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(page + 1)}
                  disabled={page >= pagination.totalPages}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
