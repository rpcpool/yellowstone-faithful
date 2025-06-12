"use client"

import {
  Clock,
  Code,
  Database,
  GalleryVerticalEnd,
  Home,
  Settings2
} from "lucide-react"
import * as React from "react"

import { NavMain } from "@/components/nav-main"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar"

// Navigation data
const data = {
  navMain: [
    {
      title: "Home",
      url: "/",
      icon: Home,
      isActive: true,
    },
    {
      title: "RPC",
      url: "/rpc",
      icon: Code,
      isActive: true,
    },
    {
      title: "Epochs",
      url: "/epochs",
      icon: Clock,
      items: [
        {
          title: "All Epochs",
          url: "/epochs",
        },
        {
          title: "Processing Status",
          url: "/epochs?status=NotProcessed",
        },
        {
          title: "Completed",
          url: "/epochs?status=Complete",
        },
        {
          title: "Indexed",
          url: "/epochs?status=Indexed",
        },
      ],
    },
    {
      title: "Data Management",
      url: "#",
      icon: Database,
      items: [
        {
          title: "Indexes",
          url: "/data/indexes",
        },
        {
          title: "Storage",
          url: "/storage",
        },
        {
          title: "GSFA Files",
          url: "/gsfa",
        },
        {
          title: "Remote Files",
          url: "/remote",
        },
      ],
    },
    {
      title: "System",
      url: "#",
      icon: Settings2,
      items: [
        {
          title: "Settings",
          url: "/settings",
        },
        {
          title: "Re-indexing",
          url: "/settings#reindex",
        },
        {
          title: "Jobs",
          url: "/settings/jobs",
        },
        {
          title: "Workers",
          url: "/workers",
        },
        {
          title: "Logs",
          url: "/logs",
        },
      ],
    },
  ],
}

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const { state } = useSidebar()
  
  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <div className="flex items-center gap-2 px-2 py-2">
          <GalleryVerticalEnd className="h-6 w-6" />
          {state === "expanded" && (
            <span className="text-sm font-semibold">Old Reliable</span>
          )}
        </div>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={data.navMain} />
      </SidebarContent>
      <SidebarFooter>
        <div className="px-2 py-2 text-center">
          {state === "expanded" && (
            <span className="text-xs text-muted-foreground">Old Reliable</span>
          )}
        </div>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
