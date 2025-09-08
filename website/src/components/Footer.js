import { Link } from 'react-router-dom';

function Footer() {
  return (
    <footer className="bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 py-16 px-5 border-t border-slate-200 dark:border-slate-700">
      <div className="max-w-6xl mx-auto grid grid-cols-1 md:grid-cols-3 gap-12">
        <div className="text-center md:text-left">
          <h3 className="text-blue-600 dark:text-blue-400 text-xl font-bold mb-6">Neurocrow</h3>
          <p className="text-slate-600 dark:text-slate-400 leading-relaxed">Transformando la comunicación con IA</p>
        </div>
        <div className="text-center md:text-left">
          <h3 className="text-blue-600 dark:text-blue-400 text-xl font-bold mb-6">Enlaces</h3>
          <ul className="space-y-3">
            <li>
              <Link 
                to="/terminos-de-servicio" 
                className="text-slate-600 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
              >
                Términos de Servicio
              </Link>
            </li>
            <li>
              <Link 
                to="/politica-de-privacidad" 
                className="text-slate-600 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
              >
                Política de Privacidad
              </Link>
            </li>
          </ul>
        </div>
        <div className="text-center md:text-left">
          <h3 className="text-blue-600 dark:text-blue-400 text-xl font-bold mb-6">Contacto</h3>
          <p className="text-slate-600 dark:text-slate-400 mb-6">Email: info@neurocrow.com</p>
          <div className="flex justify-center md:justify-start gap-6">
            <a 
              href="https://www.facebook.com/profile.php?id=61568595868220" 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-slate-600 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 text-2xl transition-colors"
            >
              <i className="fab fa-facebook"></i>
            </a>
            <a 
              href="https://instagram.com/neurocrow" 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-slate-600 dark:text-slate-400 hover:text-purple-600 dark:hover:text-purple-400 text-2xl transition-colors"
            >
              <i className="fab fa-instagram"></i>
            </a>
            <a 
              href="https://linkedin.com/company/neurocrow" 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-slate-600 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 text-2xl transition-colors"
            >
              <i className="fab fa-linkedin"></i>
            </a>
          </div>
        </div>
      </div>
      <div className="max-w-6xl mx-auto mt-12 pt-8 border-t border-slate-200 dark:border-slate-700 text-center">
        <p className="text-slate-600 dark:text-slate-400">&copy; 2024 Neurocrow. Todos los derechos reservados.</p>
      </div>
    </footer>
  );
}

export default Footer; 