
import { SidebarTrigger } from "@/components/ui/sidebar";
import Link from "next/link";

export function Header() {
  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-14 items-center">
        <SidebarTrigger className="mr-4" />
        <div className="mr-4 flex">
          <Link className="mr-6 flex items-center space-x-2" href="/">
            <div className="h-6 w-6 bg-primary rounded-sm flex items-center justify-center">
              {/*  TODO: Add logo here */}
            </div>
            <span className="hidden font-bold sm:inline-block text-foreground">
              Old Reliable
            </span>
          </Link>
        </div>
      </div>
    </header>
  );
} 