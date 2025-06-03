import { Link } from 'react-router-dom';
import './Footer.css';

function Footer() {
  return (
    <footer className="footer">
      <div className="footer-content">
        <div className="footer-section">
          <h3>Neurocrow</h3>
          <p>Transformando la comunicación con IA</p>
        </div>
        <div className="footer-section">
          <h3>Enlaces</h3>
          <ul>
            <li><Link to="/terminos-de-servicio">Términos de Servicio</Link></li>
            <li><Link to="/politica-de-privacidad">Política de Privacidad</Link></li>
          </ul>
        </div>
        <div className="footer-section">
          <h3>Contacto</h3>
          <p>Email: info@neurocrow.com</p>
          <div className="social-links">
            <a href="https://www.facebook.com/profile.php?id=61568595868220" target="_blank" rel="noopener noreferrer">
              <i className="fab fa-facebook"></i>
            </a>
            <a href="https://instagram.com/neurocrow" target="_blank" rel="noopener noreferrer">
              <i className="fab fa-instagram"></i>
            </a>
            <a href="https://linkedin.com/company/neurocrow" target="_blank" rel="noopener noreferrer">
              <i className="fab fa-linkedin"></i>
            </a>
          </div>
        </div>
      </div>
      <div className="footer-bottom">
        <p>&copy; 2024 Neurocrow. Todos los derechos reservados.</p>
      </div>
    </footer>
  );
}

export default Footer; 