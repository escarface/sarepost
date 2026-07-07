# Funcionalidades de LinkedIn en PostFlow

Este documento resume las capacidades de la aplicación cuando la red social objetivo es únicamente LinkedIn.

## Visión general

PostFlow soporta LinkedIn como un canal de publicación de primer nivel dentro del producto. El alcance actual cubre conexión de cuentas, creación de borradores, validación, programación, publicación, workflow editorial, gestión de contenido multimedia y automatización desde la Web UI, la API HTTP, MCP y la CLI.

El objetivo es permitir que un equipo opere su publicación en LinkedIn desde un único flujo de trabajo, sin depender de procesos manuales separados para cada caso.

## Tipos de cuenta de LinkedIn soportados

PostFlow soporta:

- perfiles personales de LinkedIn
- páginas de empresa u organización en LinkedIn

Las cuentas pueden conectarse mediante OAuth. En el caso de LinkedIn, el producto distingue entre:

- `personal`: conexión orientada a perfil personal
- `organization`: conexión orientada a empresa o página con permisos de organización

Esto permite publicar tanto como perfil individual como desde una página de empresa, según la cuenta conectada.

## Capacidades principales de publicación

Para LinkedIn, la aplicación soporta actualmente:

- crear borradores
- editar el texto antes de publicar
- validar un borrador antes de guardarlo o programarlo
- programar una publicación para una fecha y hora futuras
- publicar mediante el worker en segundo plano
- cancelar publicaciones programadas
- listar publicaciones programadas, borradores, publicadas y fallidas

Estas capacidades se exponen de forma consistente en las principales superficies operativas:

- Web UI
- API HTTP
- herramientas MCP para agentes de IA
- CLI

## Formatos de publicación de LinkedIn cubiertos actualmente

### 1. Publicaciones de texto con soporte para vista previa de enlace

Si una publicación de LinkedIn contiene una URL y no tiene contenido multimedia adjunto, PostFlow puede intentar una publicación de tipo artículo en el momento de publicar para que LinkedIn renderice la publicación como contenido basado en enlace.

Esto resulta útil para:

- promoción de artículos de una web
- distribución de entradas de blog
- difusión de landing pages
- publicación de noticias o anuncios con enlace

### 2. Publicaciones con imágenes

PostFlow soporta publicaciones de LinkedIn con imágenes adjuntas.

Comportamiento actual:

- hasta 9 imágenes por publicación
- las imágenes se suben antes de publicar
- el resultado se publica como un post multimedia de LinkedIn

### 3. Publicaciones con vídeo

PostFlow soporta publicaciones de LinkedIn con vídeo adjunto.

Comportamiento actual:

- un único vídeo por publicación
- el vídeo puede publicarse como recurso multimedia principal

### 4. Publicación raíz más comentarios de seguimiento

PostFlow soporta flujos de publicación en varios segmentos para LinkedIn donde:

- el primer segmento es la publicación raíz
- los segmentos siguientes se publican como comentarios sobre esa publicación raíz

Esto resulta útil para patrones de publicación como:

- publicación principal más comentario con CTA
- publicación principal más contexto adicional
- publicación principal más enlaces de apoyo en comentarios

## Funcionalidades editoriales y de planificación relevantes para LinkedIn

La publicación en LinkedIn no se limita a programar un post. También forma parte de la capa editorial de la aplicación.

El producto soporta:

- seguimiento de estados editoriales como `idea`, `drafting`, `needs_review` y `approved`
- requisitos opcionales de aprobación antes de programar
- asociación de publicaciones de LinkedIn a campañas
- gestión y filtrado de backlog
- generación de content plans que pueden incluir LinkedIn como uno de los canales seleccionados

Esto hace que LinkedIn pueda usarse tanto para publicaciones puntuales como para una operativa editorial recurrente.

## Workflow asistido por IA para LinkedIn

PostFlow incluye generación con IA en la Web UI, y ese flujo puede utilizarse para preparar contenido para LinkedIn.

Las capacidades más relevantes incluyen:

- generación de texto para publicaciones
- generación de imágenes para publicaciones
- aplicación de perfiles de marca reutilizables
- uso opcional de búsqueda web cuando el flujo de generación configurado necesita información reciente

El resultado generado puede enviarse a un borrador y continuar después por el flujo normal de publicación en LinkedIn.

## Gestión de multimedia y activos para LinkedIn

Para LinkedIn, PostFlow soporta:

- subida y almacenamiento de archivos multimedia dentro de la aplicación
- adjuntar multimedia a un borrador
- validación de combinaciones multimedia compatibles con LinkedIn
- publicación mediante lógica específica del proveedor de LinkedIn

Esto permite preparar activos una sola vez y reutilizarlos dentro del flujo de programación.

## Fiabilidad operativa

Para la operativa de LinkedIn, la aplicación también ofrece:

- validación previa a la publicación
- ejecución programada mediante el worker
- visibilidad de publicaciones fallidas a través de la cola de fallos
- reencolado de trabajos fallidos
- almacenamiento cifrado de credenciales

Esto es importante para un uso en producción donde la publicación en LinkedIn debe ser trazable y recuperable.

## Límites y reglas actuales específicas de LinkedIn

La implementación actual incluye varias restricciones explícitas:

- máximo 9 imágenes por publicación de LinkedIn
- máximo 1 vídeo por publicación de LinkedIn
- no se pueden mezclar imágenes y vídeo en una misma publicación
- los comentarios de seguimiento en LinkedIn no soportan multimedia en la versión actual
- si una publicación contiene a la vez multimedia y una URL, la multimedia tiene prioridad y se omite la vista previa del enlace

Estas reglas conviene comunicarlas de forma clara a usuarios y operadores porque afectan a cómo debe prepararse el contenido para LinkedIn.

## Casos de uso prácticos

PostFlow puede utilizarse para flujos centrados en LinkedIn como:

- programar publicaciones de fundador, directivo o marca personal
- publicar anuncios desde una página de empresa
- distribuir artículos y enlaces web
- gestionar un proceso editorial de revisión para contenido de LinkedIn
- crear calendarios editoriales de LinkedIn basados en campañas
- permitir que agentes de IA redacten, validen y programen publicaciones de LinkedIn a través de MCP

## Resumen

Para LinkedIn en concreto, PostFlow ya cubre el núcleo operativo completo:

- conexión de cuentas
- creación de borradores
- validación
- gestión multimedia
- programación
- publicación
- comentarios de seguimiento
- revisión editorial
- planificación por campañas
- automatización mediante API, MCP y CLI

En la práctica, esto significa que la aplicación ya puede funcionar como un flujo de publicación dedicado a LinkedIn, tanto en modo manual como asistido por IA.
