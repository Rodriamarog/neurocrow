// Success.js
import React, { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';

function Success() {
  const [syncStatus, setSyncStatus] = useState('syncing');
  const [setupProgress, setSetupProgress] = useState({
    pageConnection: 'pending',  // pending, success, error
    webhookSetup: 'pending',    // pending, success, error  
    handoverConfig: 'pending'   // pending, success, error
  });
  const location = useLocation();
  const accessToken = location.state?.accessToken;
  const authType = location.state?.authType || 'facebook';

  useEffect(() => {
    if (!accessToken) {
      console.error('No access token available');
      setSyncStatus('error');
      setSetupProgress({
        pageConnection: 'error',
        webhookSetup: 'error', 
        handoverConfig: 'error'
      });
      return;
    }

    // Update progress indicators step by step
    setSetupProgress(prev => ({ ...prev, pageConnection: 'in_progress' }));

    // Send token to your backend (different endpoints for each auth type)
    const endpoint = authType === 'instagram' 
      ? 'https://neurocrow-client-manager.onrender.com/instagram-token'
      : authType === 'facebook_business'
      ? 'https://neurocrow-client-manager.onrender.com/facebook-business-token'
      : 'https://neurocrow-client-manager.onrender.com/facebook-token';
    
    fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ userToken: accessToken }),
    })
    .then(response => {
      if (!response.ok) {
        throw new Error('Network response was not ok');
      }
      return response.json();
    })
    .then(data => {
      if (data.success && data.session_token) {
        // Store session token for authenticated requests
        localStorage.setItem('session_token', data.session_token);
        localStorage.setItem('client_id', data.client_id);
        localStorage.setItem('facebook_connected', 'true');
        
        // Simulate progress through the setup steps
        setSetupProgress(prev => ({ ...prev, pageConnection: 'success' }));
        
        // Simulate webhook setup (in reality this happens in backend)
        setTimeout(() => {
          setSetupProgress(prev => ({ ...prev, webhookSetup: 'in_progress' }));
          
          setTimeout(() => {
            setSetupProgress(prev => ({ ...prev, webhookSetup: 'success' }));
            
            // Simulate handover protocol setup
            setTimeout(() => {
              setSetupProgress(prev => ({ ...prev, handoverConfig: 'in_progress' }));
              
              setTimeout(() => {
                setSetupProgress(prev => ({ ...prev, handoverConfig: 'success' }));
                setSyncStatus('success');
              }, 1000);
            }, 1000);
          }, 1500);
        }, 1000);
      } else {
        throw new Error('Authentication failed - no session token received');
      }
    })
    .catch(error => {
      console.error('Error syncing pages:', error);
      setSyncStatus('error');
      setSetupProgress(prev => ({
        pageConnection: prev.pageConnection === 'in_progress' ? 'error' : prev.pageConnection,
        webhookSetup: 'error',
        handoverConfig: 'error'
      }));
    });
  }, [accessToken, authType]);

  const handleContactClick = () => {
    window.open('https://m.me/413548765185533', '_blank');
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'pending':
        return <i className="fas fa-clock text-slate-400"></i>;
      case 'in_progress':
        return <i className="fas fa-spinner fa-spin text-blue-500"></i>;
      case 'success':
        return <i className="fas fa-check-circle text-green-500"></i>;
      case 'error':
        return <i className="fas fa-times-circle text-red-500"></i>;
      default:
        return <i className="fas fa-clock text-slate-400"></i>;
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-900 flex items-center justify-center p-4">
      <div className="w-full max-w-lg bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 p-8 space-y-6 text-center rounded-xl shadow-lg">
        {syncStatus === 'syncing' ? (
          <>
            <i className="fas fa-spinner fa-spin text-6xl text-blue-500 mb-6"></i>
            <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-4">Configurando tu cuenta...</h1>
            <p className="text-slate-600 dark:text-slate-300 mb-6 leading-relaxed">Estamos configurando automáticamente tu cuenta para que funcione con Neurocrow.</p>
            
            <div className="space-y-4 text-left">
              <div className="flex items-center space-x-3">
                {getStatusIcon(setupProgress.pageConnection)}
                <span className="text-slate-700 dark:text-slate-300">Conectando tu cuenta...</span>
              </div>
              <div className="flex items-center space-x-3">
                {getStatusIcon(setupProgress.webhookSetup)}
                <span className="text-slate-700 dark:text-slate-300">Configurando mensajería automática...</span>
              </div>
              <div className="flex items-center space-x-3">
                {getStatusIcon(setupProgress.handoverConfig)}
                <span className="text-slate-700 dark:text-slate-300">Finalizando configuración...</span>
              </div>
            </div>
          </>
        ) : syncStatus === 'success' ? (
          <>
            <i className="fas fa-check-circle text-6xl text-green-500 mb-6"></i>
            <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-4">¡Configuración Completada!</h1>
            <p className="text-slate-600 dark:text-slate-300 mb-6 leading-relaxed">Tu cuenta ha sido configurada automáticamente. Tus páginas ya están listas para recibir mensajes y usar el chatbot de Neurocrow.</p>
            
          </>
        ) : (
          <>
            <i className="fas fa-exclamation-circle text-6xl text-red-500 mb-6"></i>
            <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-4">Hubo un problema</h1>
            <p className="text-slate-600 dark:text-slate-300 mb-6 leading-relaxed">No pudimos completar la configuración automática. Por favor contáctanos para ayudarte.</p>
          </>
        )}
        
        <div className="mt-8">
          <p className="text-slate-600 dark:text-slate-300 mb-4">Si tienes alguna pregunta, puedes contactarnos por:</p>
          <div className="flex gap-4 justify-center">
            <button 
              onClick={handleContactClick} 
              className="flex items-center gap-2 px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-all hover:scale-105"
            >
              <i className="fab fa-facebook-messenger"></i> 
              Messenger
            </button>
            <a 
              href="https://wa.me/+16197612314" 
              target="_blank" 
              rel="noopener noreferrer" 
              className="flex items-center gap-2 px-6 py-3 bg-green-600 hover:bg-green-700 text-white font-medium rounded-lg transition-all hover:scale-105 no-underline"
            >
              <i className="fab fa-whatsapp"></i> 
              WhatsApp
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}

export default Success;