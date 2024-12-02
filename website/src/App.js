import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { useEffect, useState } from 'react';
import Navbar from './components/Navbar';
import Footer from './components/Footer';
import Home from './pages/Home';
import Login from './pages/Login'; 
import TerminosServicio from './pages/TerminosServicio';
import PoliticaPrivacidad from './pages/PoliticaPrivacidad';
import './index.css';

function App() {
 const [fbInitialized, setFbInitialized] = useState(false);
 const [fbError, setFbError] = useState(null);
 const [authStatus, setAuthStatus] = useState('unknown'); // 'unknown', 'connected', 'not_connected'

 useEffect(() => {
   // Initialize Facebook SDK
   if (!window.FB) {
     try {
       window.fbAsyncInit = function() {
         window.FB.init({
           appId: '1195277397801905', // Replace with your Facebook App ID
           cookie: true,
           xfbml: true,
           version: 'v18.0'
         });

         // Check login status
         window.FB.getLoginStatus(function(response) {
           if (response.status === 'connected') {
             setAuthStatus('connected');
             const accessToken = response.authResponse.accessToken;
             // You could store the token in state or localStorage here
           } else {
             setAuthStatus('not_connected');
           }
         });

         setFbInitialized(true);
       };

       // Handle SDK load error
       window.setTimeout(() => {
         if (!window.FB) {
           setFbError('Facebook SDK failed to load');
         }
       }, 5000); // Check after 5 seconds

     } catch (error) {
       setFbError(`Error initializing Facebook SDK: ${error.message}`);
     }
   }
 }, []);

 // Error display component
 const ErrorMessage = () => (
   <div style={{
     padding: '20px',
     backgroundColor: '#fee2e2',
     border: '1px solid #ef4444',
     borderRadius: '8px',
     margin: '20px',
     color: '#991b1b'
   }}>
     <h3>Error de conexión</h3>
     <p>{fbError}</p>
     <p>Por favor, recarga la página o intenta más tarde.</p>
   </div>
 );

 // Loading display component
 const LoadingMessage = () => (
   <div style={{
     padding: '20px',
     textAlign: 'center',
     margin: '20px'
   }}>
     <p>Cargando...</p>
   </div>
 );

 // Show error if FB SDK failed to load
 if (fbError) {
   return (
     <Router>
       <div className="App">
         <Navbar />
         <ErrorMessage />
         <Footer />
       </div>
     </Router>
   );
 }

 // Show loading while FB SDK initializes
 if (!fbInitialized) {
   return (
     <Router>
       <div className="App">
         <Navbar />
         <LoadingMessage />
         <Footer />
       </div>
     </Router>
   );
 }

 return (
   <Router>
     <div className="App">
       <Navbar />
       <main>
         <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/login" element={<Login />} /> {/* Add this line */}
            <Route path="/terminos-de-servicio" element={<TerminosServicio />} />
            <Route path="/politica-de-privacidad" element={<PoliticaPrivacidad />} />
         </Routes>
       </main>
       <Footer />
     </div>
   </Router>
 );
}

export default App;