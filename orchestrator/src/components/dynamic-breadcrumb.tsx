"use client"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { generateBreadcrumbs } from "@/lib/breadcrumbs"
import Link from "next/link"
import { usePathname, useSearchParams } from "next/navigation"
import React from "react"

export function DynamicBreadcrumb() {
  const pathname = usePathname()
  const searchParams = useSearchParams()
  
  const breadcrumbs = generateBreadcrumbs(pathname, searchParams)
  
  if (breadcrumbs.length <= 1) {
    // Show simple title for home page or single-level pages
    const currentPage = breadcrumbs[0]?.label || "Home"
    return (
      <h1 className="text-base font-medium">{currentPage}</h1>
    )
  }
  
  return (
    <Breadcrumb>
      <BreadcrumbList>
        {breadcrumbs.map((breadcrumb, index) => (
          <React.Fragment key={`${breadcrumb.href || breadcrumb.label}-${index}`}>
            <BreadcrumbItem>
              {breadcrumb.isCurrentPage ? (
                <BreadcrumbPage>{breadcrumb.label}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link href={breadcrumb.href!}>{breadcrumb.label}</Link>
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
            {index < breadcrumbs.length - 1 && <BreadcrumbSeparator />}
          </React.Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
} 