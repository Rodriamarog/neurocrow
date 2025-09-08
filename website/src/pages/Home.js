import demoGif from '../assets/demo.gif';

function Home() {
 const handleContactClick = () => {
   window.open('https://m.me/413548765185533', '_blank');
 };

 return (
   <div className="min-h-screen bg-slate-50 dark:bg-slate-900">
     {/* Hero Section */}
     <section className="relative bg-gradient-to-br from-blue-50 via-white to-purple-50 dark:from-slate-800 dark:via-slate-900 dark:to-slate-800 py-20 lg:py-32">
       <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
         <div className="text-center space-y-8">
           <h1 className="text-4xl lg:text-6xl font-bold text-slate-900 dark:text-slate-100 leading-tight">
             Automatiza todas tus{' '}
             <span className="bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
               Redes Sociales
             </span>
           </h1>
           <p className="text-xl lg:text-2xl text-slate-600 dark:text-slate-300 max-w-3xl mx-auto leading-relaxed">
             Integración perfecta con Facebook Messenger, Instagram y WhatsApp
           </p>
           <div className="relative max-w-4xl mx-auto">
             <div className="bg-white dark:bg-slate-800 rounded-2xl shadow-2xl p-4 border border-slate-200 dark:border-slate-700">
               <img 
                 src={demoGif} 
                 alt="Demo del chatbot"
                 className="w-full h-auto rounded-xl"
               />
             </div>
           </div>
         </div>
       </div>
     </section>

     {/* Services Section */}
     <section className="py-20 lg:py-32 bg-white dark:bg-slate-800">
       <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
         <div className="text-center mb-16">
           <h2 className="text-3xl lg:text-4xl font-bold text-slate-900 dark:text-slate-100 mb-4">
             Nuestros Servicios
           </h2>
           <div className="w-24 h-1 bg-gradient-to-r from-blue-500 to-purple-500 mx-auto rounded-full"></div>
         </div>
         
         <div className="grid grid-cols-1 md:grid-cols-2 gap-12">
           <div className="text-center p-8 bg-slate-50 dark:bg-slate-700 rounded-2xl border border-slate-200 dark:border-slate-600">
             <div className="w-20 h-20 bg-blue-600 rounded-2xl mx-auto mb-6 flex items-center justify-center">
               <i className="fab fa-facebook text-3xl text-white"></i>
             </div>
             <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-4">Facebook Messenger</h3>
             <p className="text-slate-600 dark:text-slate-300 leading-relaxed">
               Chatbots personalizados para tu página de Facebook con respuestas automáticas inteligentes
             </p>
           </div>
           
           <div className="text-center p-8 bg-slate-50 dark:bg-slate-700 rounded-2xl border border-slate-200 dark:border-slate-600">
             <div className="w-20 h-20 bg-gradient-to-r from-purple-500 to-pink-500 rounded-2xl mx-auto mb-6 flex items-center justify-center">
               <i className="fab fa-instagram text-3xl text-white"></i>
             </div>
             <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-4">Instagram</h3>
             <p className="text-slate-600 dark:text-slate-300 leading-relaxed">
               Automatiza tus respuestas en Instagram Direct con mensajería inteligente
             </p>
           </div>
         </div>
       </div>
     </section>

     {/* CTA Section */}
     <section className="py-20 lg:py-32 bg-gradient-to-r from-blue-600 to-purple-600">
       <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
         <h2 className="text-3xl lg:text-4xl font-bold text-white mb-6 leading-tight">
           ¿Listo para revolucionar tu atención al cliente?
         </h2>
         <button 
           className="inline-flex items-center gap-3 px-8 py-4 bg-white text-blue-600 font-bold text-lg rounded-xl hover:bg-slate-100 transition-all duration-300 shadow-xl hover:scale-105"
           onClick={handleContactClick}
         >
           <i className="fab fa-facebook-messenger text-xl"></i>
           Contáctanos
         </button>
       </div>
     </section>
   </div>
 );
}

export default Home;