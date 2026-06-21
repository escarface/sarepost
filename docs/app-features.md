# Características Fundamentales de Sarepost

Este documento resume, de forma ejecutiva, las capacidades principales de la aplicación Sarepost.

## Visión general

Sarepost es una aplicación de publicación social autoalojada que permite planificar, generar, programar y publicar contenido desde un único lugar. Combina una interfaz web para operación diaria con API, MCP y CLI para automatización y flujos asistidos por IA.

## Funcionalidades principales

### 1. Publicación multicanal

Sarepost permite trabajar con varias plataformas sociales desde una misma aplicación:

- X (Twitter)
- LinkedIn
- Facebook
- Instagram

Las cuentas pueden conectarse mediante OAuth y después utilizarse en la creación, programación y publicación de contenidos.

### 2. Creación y programación de publicaciones

La aplicación permite:

- crear borradores
- editar texto y material multimedia
- programar publicaciones para una fecha y hora futuras
- publicar en el momento cuando el flujo lo permite
- cancelar publicaciones programadas

Además, incluye validaciones y reglas de control para reducir errores operativos, duplicidades y conflictos de calendario.

### 3. Generación de contenido con IA

Sarepost incorpora una vista de generación que permite crear:

- texto para publicaciones
- imágenes generadas por IA

El contenido generado puede enviarse directamente a un nuevo borrador, manteniendo el flujo de trabajo dentro de la propia aplicación.

### 4. Perfiles de marca reutilizables

La generación con IA puede apoyarse en perfiles de marca para mantener coherencia en el contenido. Cada perfil puede incluir:

- contexto de marca
- prompt de sistema
- tono
- imagen de referencia para estilo visual

Esto permite que el texto y las imágenes se adapten mejor a la identidad de cada marca o cliente.

### 5. Generación con búsqueda web en tiempo real

Cuando el prompt lo requiere, Sarepost puede usar búsqueda web a través del proveedor de IA configurado para generar contenido basado en información reciente.

Esto resulta especialmente útil para:

- noticias de actualidad
- tendencias recientes
- resúmenes semanales
- publicaciones basadas en hechos recientes

La aplicación puede indicar si se ha usado búsqueda web y mostrar las fuentes empleadas en la respuesta.

### 6. Workflow editorial

Sarepost no es solo un programador de posts. También incorpora una capa editorial con funcionalidades como:

- estados editoriales
- aprobación obligatoria antes de programar
- asociación de publicaciones a campañas
- gestión de backlog

Esto permite controlar el ciclo completo de contenido desde la idea inicial hasta la publicación.

### 7. Gestión de campañas

La aplicación permite agrupar publicaciones dentro de campañas editoriales con un briefing común. Una campaña puede incluir campos como:

- objetivo
- audiencia
- tono
- llamada a la acción
- restricciones
- etiquetas
- zona horaria

Esto facilita la planificación estratégica y la organización del calendario de contenidos.

### 8. Gestión de archivos multimedia

Sarepost permite subir y adjuntar contenido multimedia a las publicaciones, incluyendo recursos generados por IA. Los archivos quedan persistidos y pueden reutilizarse dentro del flujo de trabajo.

### 9. Gestión de errores y recuperación

Si una publicación falla, Sarepost la registra en una cola de fallos para poder revisarla, reintentarlo o volver a encolarla más tarde. Esto mejora la trazabilidad y la seguridad operativa.

### 10. Varias superficies de acceso

Sarepost está diseñado como un sistema orientado a integraciones y agentes de IA. Sus capacidades se exponen a través de varias superficies:

- interfaz web
- API HTTP
- endpoint MCP
- CLI

Esto permite operar la plataforma tanto manualmente como mediante automatizaciones o asistentes inteligentes.

## Características técnicas

- desarrollado en Go
- persistencia basada en SQLite
- secretos almacenados cifrados en reposo
- despliegue ligero y autoalojado
- arquitectura de monolito modular

## Casos de uso habituales

- programar publicaciones en varias redes desde una única herramienta
- generar contenido social alineado con una marca mediante IA
- preparar calendarios editoriales y campañas de contenido
- permitir que agentes de IA creen y gestionen publicaciones
- operar un sistema de publicación propio sin depender de herramientas externas cerradas

## Documentación relacionada

- [README.md](../README.md)
- [architecture.md](./architecture.md)
- [openapi.yaml](./specs/openapi.yaml)
