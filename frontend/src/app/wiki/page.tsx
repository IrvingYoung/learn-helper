import { useParams, useSearchParams } from 'react-router-dom';
import { WikiPageLayout } from '../../components/WikiPage';

export default function WikiPage() {
  const { slug } = useParams<{ slug?: string }>();
  const [searchParams] = useSearchParams();
  // shareToken is only meaningful on /share/:slug routes, but passing null on
  // /wiki and /wiki/:slug is harmless — the layout uses it only when
  // isPublicPath is true.
  const shareToken = searchParams.get('t');
  return <WikiPageLayout urlSlug={slug ?? null} shareTokenFromUrl={shareToken} />;
}
