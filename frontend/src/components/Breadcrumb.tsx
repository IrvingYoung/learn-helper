import { Link } from "react-router";

interface Crumb { slug: string; name: string; }

interface BreadcrumbProps {
  items: Crumb[];
}

export function Breadcrumb({ items }: BreadcrumbProps) {
  return (
    <nav className="flex items-center gap-1 text-sm text-gray-500 mb-4">
      {items.map((item, i) => (
        <span key={item.slug} className="flex items-center gap-1">
          {i > 0 && <span className="text-gray-300">/</span>}
          <Link to={`/learn/${item.slug}`} className="hover:text-blue-600 transition-colors">{item.name}</Link>
        </span>
      ))}
    </nav>
  );
}
