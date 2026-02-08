import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import { ThemeProvider } from './contexts/ThemeContext'
import { WebAuthWrapper } from './WebAuthWrapper'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ThemeProvider>
            <WebAuthWrapper>
                <App/>
            </WebAuthWrapper>
        </ThemeProvider>
    </React.StrictMode>
)
