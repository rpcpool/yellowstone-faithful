export interface BreadcrumbItem {
  label: string
  href?: string
  isCurrentPage?: boolean
}

// Route mapping for breadcrumb labels
const routeLabels: Record<string, string> = {
  '/': 'Home',
  '/dashboard': 'Statistics',
  '/epochs': 'Epochs',
  '/indexes': 'Indexes',
  '/storage': 'Storage',
  '/gsfa': 'GSFA Files',
  '/remote': 'Remote Files',
  '/settings': 'Settings',
  '/workers': 'Workers',
  '/logs': 'Logs',
  '/analytics': 'Analytics',
  '/rpc': 'RPC',
}

// Special handling for dynamic routes
const dynamicRouteLabels: Record<string, (segment: string) => string> = {
  '/epochs/[id]': (id: string) => `Epoch ${id}`,
  '/indexes/[id]': (id: string) => `Index ${id}`,
}

export function generateBreadcrumbs(pathname: string, searchParams?: URLSearchParams): BreadcrumbItem[] {
  const breadcrumbs: BreadcrumbItem[] = []
  
  // Always start with Home
  if (pathname !== '/') {
    breadcrumbs.push({
      label: 'Home',
      href: '/',
    })
  }
  
  // Split pathname into segments
  const segments = pathname.split('/').filter(Boolean)
  
  if (segments.length === 0) {
    // We're on the home page
    breadcrumbs.push({
      label: 'Home',
      isCurrentPage: true,
    })
    return breadcrumbs
  }
  
  // Build breadcrumbs for each segment
  let currentPath = ''
  
  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i]
    currentPath += `/${segment}`
    const isLast = i === segments.length - 1
    
    let label = segment
    
    // Check if this is a known route
    if (routeLabels[currentPath]) {
      label = routeLabels[currentPath]
    } else {
      // Check for dynamic routes
      const parentPath = segments.slice(0, i).join('/')
      const dynamicPattern = `${parentPath ? '/' + parentPath : ''}/[id]`
      
      if (dynamicRouteLabels[dynamicPattern]) {
        label = dynamicRouteLabels[dynamicPattern](segment)
      } else {
        // Check if this looks like a dynamic ID (numeric or specific patterns)
        const isNumeric = /^\d+$/.test(segment)
        if (isNumeric && i > 0) {
          // This is likely a dynamic ID, use the parent route to determine the label
          const parentRoute = `/${segments.slice(0, i).join('/')}`
          if (parentRoute === '/epochs') {
            label = `Epoch ${segment}`
          } else if (parentRoute === '/indexes') {
            label = `Index ${segment}`
          } else {
            label = `ID ${segment}`
          }
        } else {
          // Fallback: capitalize and format the segment
          label = segment
            .split('-')
            .map(word => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ')
        }
      }
    }
    
    breadcrumbs.push({
      label,
      href: isLast ? undefined : currentPath,
      isCurrentPage: isLast,
    })
  }
  
  // Add query parameter context for certain pages
  if (pathname === '/epochs' && searchParams) {
    const status = searchParams.get('status')
    if (status && status !== 'all') {
      const statusLabels: Record<string, string> = {
        'NotProcessed': 'Not Processed',
        'Complete': 'Completed',
        'Indexed': 'Indexed',
      }
      
      // Add a separate filter breadcrumb
      if (statusLabels[status]) {
        breadcrumbs.push({
          label: `Filter: ${statusLabels[status]}`,
          isCurrentPage: true,
        })
        
        // Update the previous breadcrumb to not be the current page
        const previousBreadcrumb = breadcrumbs[breadcrumbs.length - 2]
        if (previousBreadcrumb) {
          previousBreadcrumb.isCurrentPage = false
          previousBreadcrumb.href = pathname
        }
      }
    }
  }
  
  return breadcrumbs
} 