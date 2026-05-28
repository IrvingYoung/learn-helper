import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import LearnPage from './app/learn/page'
import TopicDetailPage from './app/learn/[slug]/page'
import PracticePage from './app/practice/page'
import DashboardPage from './app/dashboard/page'
import SettingsPage from './app/settings/page'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/learn" replace />} />
          <Route path="learn" element={<LearnPage />} />
          <Route path="learn/:slug" element={<TopicDetailPage />} />
          <Route path="practice" element={<PracticePage />} />
          <Route path="practice/:id" element={<PracticePage />} />
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}