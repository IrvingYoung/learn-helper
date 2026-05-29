import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import WikiPage from './app/wiki/page'
import SettingsPage from './app/settings/page'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/wiki" replace />} />
        <Route path="/wiki" element={<WikiPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Routes>
    </BrowserRouter>
  )
}
