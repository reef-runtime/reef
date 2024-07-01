// if (!process.env.API_URL) {
//   throw new Error('API_URL environment variable is required');
// }

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  trailingSlash: true,
  images: { unoptimized: true },
//   rewrites: async () => {
//     return [
//       {
//         source: '/api/:path*',
//         destination: `${process.env.API_URL}/:path*`,
//       },
//     ];
//   },
};

export default nextConfig;
