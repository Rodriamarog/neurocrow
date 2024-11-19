import { useState } from 'react';
import { Link } from 'react-router-dom';
import './Navbar.css';

function Navbar() {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <nav className="navbar">
      <div className="navbar-container">
        <Link to="/" className="navbar-logo">
          Neurocrow
        </Link>
        
        <div className="menu-icon" onClick={() => setIsOpen(!isOpen)}>
          <i className={isOpen ? 'fas fa-times' : 'fas fa-bars'} />
        </div>

        <ul className={isOpen ? 'nav-menu active' : 'nav-menu'}>
          <li className="nav-item">
            <Link to="/" className="nav-link" onClick={() => setIsOpen(false)}>
              Inicio
            </Link>
          </li>
          <li className="nav-item">
            <Link to="/terminos-de-servicio" className="nav-link" onClick={() => setIsOpen(false)}>
              Términos de Servicio
            </Link>
          </li>
          <li className="nav-item">
            <Link to="/politica-de-privacidad" className="nav-link" onClick={() => setIsOpen(false)}>
              Política de Privacidad
            </Link>
          </li>
        </ul>
      </div>
    </nav>
  );
}

export default Navbar; 