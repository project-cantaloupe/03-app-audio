import { lazy, Suspense, type ReactNode } from "react";
import { createBrowserRouter } from "react-router-dom";
import { AppShell } from "../components/layout/AppShell";
import { NotFoundPage } from "../pages/NotFoundPage";

const LandingPage = lazy(() => import("../pages/LandingPage").then((module) => ({ default: module.LandingPage })));
const DiscoverPage = lazy(() => import("../pages/DiscoverPage").then((module) => ({ default: module.DiscoverPage })));
const SearchPage = lazy(() => import("../pages/SearchPage").then((module) => ({ default: module.SearchPage })));
const TrackPage = lazy(() => import("../pages/TrackPage").then((module) => ({ default: module.TrackPage })));
const CreatorPage = lazy(() => import("../pages/CreatorPage").then((module) => ({ default: module.CreatorPage })));
const PlaylistPage = lazy(() => import("../pages/PlaylistPage").then((module) => ({ default: module.PlaylistPage })));
const LibraryPage = lazy(() => import("../pages/LibraryPage").then((module) => ({ default: module.LibraryPage })));
const UploadPage = lazy(() => import("../pages/UploadPage").then((module) => ({ default: module.UploadPage })));
const NotificationsPage = lazy(() => import("../pages/NotificationsPage").then((module) => ({ default: module.NotificationsPage })));
const SettingsPage = lazy(() => import("../pages/SettingsPage").then((module) => ({ default: module.SettingsPage })));
const SignInPage = lazy(() => import("../pages/SignInPage").then((module) => ({ default: module.SignInPage })));
const AuthCallbackPage = lazy(() => import("../pages/AuthCallbackPage").then((module) => ({ default: module.AuthCallbackPage })));

function load(element: ReactNode) {
  return <Suspense fallback={<div className="route-loading" role="status">Tuning the signal…</div>}>{element}</Suspense>;
}

export const router = createBrowserRouter([
  { path: "/", element: load(<LandingPage />) },
  { path: "/landing", element: load(<LandingPage />) },
  { path: "/sign-in", element: load(<SignInPage />) },
  { path: "/auth/callback", element: load(<AuthCallbackPage />) },
  {
    element: <AppShell />,
    children: [
      { path: "/discover", element: load(<DiscoverPage />) },
      { path: "/search", element: load(<SearchPage />) },
      { path: "/track/:trackId", element: load(<TrackPage />) },
      { path: "/creator/:creatorId", element: load(<CreatorPage />) },
      { path: "/playlist/:playlistId", element: load(<PlaylistPage />) },
      { path: "/library", element: load(<LibraryPage />) },
      { path: "/upload", element: load(<UploadPage />) },
      { path: "/notifications", element: load(<NotificationsPage />) },
      { path: "/settings", element: load(<SettingsPage />) },
    ],
  },
  { path: "*", element: <NotFoundPage /> },
]);
