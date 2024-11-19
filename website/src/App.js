import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import Navbar from './components/Navbar';
import Footer from './components/Footer';
import Home from './pages/Home';
import TerminosServicio from './pages/TerminosServicio';
import PoliticaPrivacidad from './pages/PoliticaPrivacidad';

function App() {
  return (
    <Router>
      <div className="App">
        <Navbar />
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/terminos-de-servicio" element={<TerminosServicio />} />
          <Route path="/politica-de-privacidad" element={<PoliticaPrivacidad />} />
        </Routes>
        <Footer />
      </div>
    </Router>
  );
}

export default App;
