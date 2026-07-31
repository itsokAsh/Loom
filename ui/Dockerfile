FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG VITE_API_URL=/v1
ARG VITE_ADMIN_API_KEY=dev-admin-key
ARG VITE_WEBHOOK_BASE=/v1/webhooks
ENV VITE_API_URL=$VITE_API_URL
ENV VITE_ADMIN_API_KEY=$VITE_ADMIN_API_KEY
ENV VITE_WEBHOOK_BASE=$VITE_WEBHOOK_BASE
RUN npm run build

FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
