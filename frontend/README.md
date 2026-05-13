This is the Pag-Asa Centre website, built with [Next.js](https://nextjs.org).

## Getting Started

1. Copy `.env.local.example` to `.env.local` and point it at the backend:
   ```bash
   cp .env.local.example .env.local
   ```
   If your backend is running on the default port (no `API_HOST_PORT` set in `backend/.env`), use `http://localhost:8080`. Otherwise match the override (e.g. `http://localhost:8081`).
2. Install dependencies and run the dev server:
   ```bash
   npm install
   npm run dev
   ```
3. Open [http://localhost:3000](http://localhost:3000).

The camp registration form lives at `/camp/register` and hits the Go backend in `../backend`. Make sure that's running (`cd ../backend && make docker-up`) before testing the submit flow.

You can start editing the page by modifying `src/app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.
