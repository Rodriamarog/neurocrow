import './Logo.css';
import { Link } from 'react-router-dom';

function Logo() {
  return (
    <Link to="/" className="logo-link">
      <div className="logo">
        <span className="logo-neuro">NEURO</span>
        <span className="logo-crow">CROW</span>
      </div>
    </Link>
  );
}

export default Logo; 