import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import WikiPage from './app/wiki/page'
import SettingsPage from './app/settings/page'
import CronListPage from './app/cron/page'
import NewCronTaskPage from './app/cron/new/page'
import EditCronTaskPage from './app/cron/[id]/page'
import AllCronRunsPage from './app/cron/runs/page'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/wiki" replace />} />
        <Route path="/wiki" element={<WikiPage />} />
        <Route path="/wiki/:slug" element={<WikiPage />} />
        <Route path="/share/:slug" element={<WikiPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/cron" element={<CronListPage />} />
        <Route path="/cron/new" element={<NewCronTaskPage />} />
        <Route path="/cron/runs" element={<AllCronRunsPage />} />
        <Route path="/cron/:id" element={<EditCronTaskPage />} />
      </Routes>
    </BrowserRouter>
  )
}
