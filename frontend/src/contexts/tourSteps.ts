export interface TourStep {
  title: string
  content: string
  target?: string
  route?: string
  placement?: 'top' | 'bottom' | 'left' | 'right' | 'center'
}

export const TOUR_STEPS: TourStep[] = [
  {
    title: 'Welcome to NetShield',
    content: 'NetShield is a high-performance DNS management server for your home network. This interactive guide will walk you through the core modules, detailing how our Go-based resolver and SQLite database function under the hood to secure and optimize your local queries.',
    placement: 'center',
    route: '/',
  },
  {
    title: 'DNS Queries Analytics',
    content: 'This area chart plots real-time traffic statistics. Cached queries are answered from local memory in sub-milliseconds. Blocked queries are intercepted by blocklists and sinkholed. Allowed queries are forwarded to upstream resolvers and cached.',
    target: '[data-tour="chart-queries"]',
    placement: 'bottom',
    route: '/',
  },
  {
    title: 'System Health Diagnostics',
    content: 'Monitors real-time resource utilization, server uptime, and active resolver status. Under the hood, the Go server coordinates system metrics and responds to settings changes instantly, eliminating the need to manually restart the DNS daemon.',
    target: '[data-tour="system-health"]',
    placement: 'bottom',
    route: '/',
  },
  {
    title: 'Navigation Menu',
    content: 'This navigation panel lets you configure local DNS records, routing rules, domain blocklists, and system preferences. Let\'s move to the DNS Records page next.',
    target: '[data-tour="sidebar-navigation"]',
    placement: 'right',
    route: '/',
  },
  {
    title: 'Local DNS Records',
    content: 'This table houses local DNS records (like A, AAAA, CNAME) which map custom domains (e.g. "nas.home") directly to local IPs. The NetShield resolver evaluates these rules first, serving local queries authoritatively and overriding upstream responses.',
    target: '[data-tour="dns-records-list"]',
    placement: 'top',
    route: '/records',
  },
  {
    title: 'Traffic Steering Policies',
    content: 'Traffic steering dynamically intercepts queries matching specific domains or client IPs. Steered queries can be forwarded to alternate DNS upstreams (e.g., secure DoH/DoT endpoints) or redirected to alternate IP destinations in priority order.',
    target: '[data-tour="traffic-steering-list"]',
    placement: 'top',
    route: '/steering',
  },
  {
    title: 'Security Blocklists',
    content: 'Enforce domain blocking policies here. Added wildcards or domains match queries instantly. When a block is active, the resolver returns 0.0.0.0 or NXDOMAIN, saving local bandwidth and shielding clients from malicious tracking domains.',
    target: '[data-tour="blocklist-list"]',
    placement: 'top',
    route: '/blocklist',
  },
  {
    title: 'Real-Time Activity Logs',
    content: 'Inspect every DNS query made by local clients. You can search by domain, filter by status (allowed, blocked, cached), and review resolution times to detect network latency bottlenecks or suspicious device activity.',
    target: '[data-tour="query-logs-list"]',
    placement: 'top',
    route: '/logs',
  },
  {
    title: 'Upstream Resolver Settings',
    content: 'Define primary DNS resolvers, minimum/maximum cache TTL limits, and toggle blocked response behavior. Returning NXDOMAIN triggers name error warnings on clients, whereas 0.0.0.0 acts as a faster, silent sinkhole.',
    target: '[data-tour="settings-card"]',
    placement: 'top',
    route: '/settings',
  },
  {
    title: 'Admin Profile Settings',
    content: 'Manage your administrator credentials. Updating your display name or email pushes changes directly to the persistent SQLite "users" table. Updates are immediately reflected across the system (such as in the top header).',
    target: '[data-tour="profile-card"]',
    placement: 'top',
    route: '/profile',
  },
  {
    title: 'Alert Notification Center',
    content: 'Alerts you of significant system events, such as blocking configuration toggles, rule updates, or password changes. Notifications are stored in the server SQLite database and polled dynamically to maintain real-time status visibility.',
    target: '[data-tour="notification-bell"]',
    placement: 'bottom',
    route: '/',
  },
  {
    title: 'NetShield Tour Completed!',
    content: 'You are now ready to operate NetShield like a pro! If you need to re-run this guide or review technical descriptions in the future, you can restart the tour at any time from the sidebar menu.',
    placement: 'center',
    route: '/',
  },
]
