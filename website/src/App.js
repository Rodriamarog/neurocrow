import { 
  createBrowserRouter, 
  RouterProvider,
  createRoutesFromElements,
  Route,
  useLocation 
} from 'react-router-dom';
import { useEffect, useState } from 'react';
import Navbar from './components/Navbar';
import Footer from './components/Footer';
import Home from './pages/Home';
import Login from './pages/Login';
import Success from './pages/Success';
import InstagramCallback from './pages/InstagramCallback';
import ContentManager from './pages/ContentManager';
import { Outlet } from 'react-router-dom';
import TerminosServicio from './pages/TerminosServicio';
import PoliticaPrivacidad from './pages/PoliticaPrivacidad';
import { DarkModeProvider } from './contexts/DarkModeContext';
import './index.css';

// Loading component
const LoadingSpinner = () => (
  <div className="min-h-screen flex items-center justify-center p-5 bg-slate-50 dark:bg-slate-900">
    <div className="p-5 text-center text-slate-700 dark:text-slate-300">Cargando...</div>
  </div>
);

// Error display component
const ErrorMessage = ({ error }) => (
  <div className="min-h-screen flex items-center justify-center p-5 bg-slate-50 dark:bg-slate-900">
    <div className="bg-red-50 dark:bg-red-900/20 border border-red-300 dark:border-red-700 rounded-lg p-5 max-w-md text-center text-red-800 dark:text-red-200">
      <h3 className="font-bold mb-2">Error de conexión</h3>
      <p className="mb-2">{error}</p>
      <p>Por favor, recarga la página o intenta más tarde.</p>
    </div>
  </div>
);

// Facebook-aware component wrapper
const FacebookAwareComponent = ({ children }) => {
  const [isFBLoading, setIsFBLoading] = useState(true);
  const [fbError, setFbError] = useState(null);
  const location = useLocation();

  useEffect(() => {
    const checkFB = setInterval(() => {
      if (window.FB) {
        try {
          window.FB.init({
            appId: '1195277397801905',
            cookie: true,
            xfbml: false,
            version: 'v18.0'
          });
          setIsFBLoading(false);
          clearInterval(checkFB);
        } catch (error) {
          setFbError(`Error initializing Facebook SDK: ${error.message}`);
          clearInterval(checkFB);
        }
      }
    }, 100);

    const timeout = setTimeout(() => {
      clearInterval(checkFB);
      if (!window.FB) {
        setFbError('Facebook SDK failed to load');
      }
      setIsFBLoading(false);
    }, 5000);

    return () => {
      clearInterval(checkFB);
      clearTimeout(timeout);
    };
  }, []);

  const isLoginPage = location.pathname === '/login';

  if (fbError && isLoginPage) {
    return <ErrorMessage error={fbError} />;
  }

  if (isFBLoading && isLoginPage) {
    return <LoadingSpinner />;
  }

  return children;
};

// Layout component to wrap all routes
const Layout = () => {
  return (
    <div className="text-center">
      <FacebookAwareComponent>
        <Navbar />
        <main>
          <Outlet />
        </main>
        <Footer />
      </FacebookAwareComponent>
    </div>
  );
};

// Create router with future flags enabled
const router = createBrowserRouter(
  createRoutesFromElements(
    <Route element={<Layout />}>
      <Route path="/" element={<Home />} />
      <Route path="/login" element={<Login />} />
      <Route path="/success" element={<Success />} />
      <Route path="/instagram-callback" element={<InstagramCallback />} />
      <Route path="/content-manager" element={<ContentManager />} />
      <Route path="/terminos-de-servicio" element={<TerminosServicio />} />
      <Route path="/politica-de-privacidad" element={<PoliticaPrivacidad />} />
    </Route>
  ),
  {
    future: {
      v7_startTransition: true,
      v7_relativeSplatPath: true
    }
  }
);

function App() {
  return (
    <DarkModeProvider>
      <RouterProvider router={router} />
    </DarkModeProvider>
  );
}

export default App;