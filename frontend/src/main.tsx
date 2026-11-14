import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import ImageLightbox from './components/ImageLightbox'
import { I18nProvider } from './i18n'
import './styles/index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider>
      <App />
      {/* 图片查看器全局只挂一次：任一页面的富文本点图都走它 */}
      <ImageLightbox />
    </I18nProvider>
  </React.StrictMode>
)
