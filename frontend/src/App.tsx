import { BrowserRouter, Route, Routes } from 'react-router-dom'
import Home from './pages/Home'
import ExplanationPage from './pages/ExplanationPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/explanations/:id" element={<ExplanationPage />} />
      </Routes>
    </BrowserRouter>
  )
}
