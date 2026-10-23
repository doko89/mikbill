import { Suspense, lazy } from "react";
import { Route, Routes } from "react-router-dom";
import { MotionConfig } from "framer-motion";
import Home from "./pages/Home";
import NotFound from "./pages/NotFound";
import AdminLogin from "./pages/admin/Login";
import { RequireAuth, SetupGuard } from "./components/guards";
import { PageSkeleton } from "./components/ui";

const Setup = lazy(() => import("./pages/Setup"));
const Help = lazy(() => import("./pages/Help"));
const Pay = lazy(() => import("./pages/Pay"));
const Voucher = lazy(() => import("./pages/Voucher"));
const AdminLayout = lazy(() => import("./pages/admin/Layout"));
const Dashboard = lazy(() => import("./pages/admin/Dashboard"));
const RouterPage = lazy(() => import("./pages/admin/Router"));
const PackagesPage = lazy(() => import("./pages/admin/Packages"));
const Transactions = lazy(() => import("./pages/admin/Transactions"));
const TransactionDetail = lazy(() => import("./pages/admin/TransactionDetail"));
const SettingsPage = lazy(() => import("./pages/admin/Settings"));

function AdminFallback() {
  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-10">
      <PageSkeleton lines={4} />
    </div>
  );
}

function SetupFallback() {
  return (
    <div className="mx-auto w-full max-w-md px-4 py-10">
      <PageSkeleton lines={5} />
    </div>
  );
}

export default function App() {
  return (
    <MotionConfig reducedMotion="user">
      <Routes>
        <Route path="/" element={<Home />} />
        <Route
          path="/bantuan"
          element={
            <Suspense fallback={<SetupFallback />}>
              <Help />
            </Suspense>
          }
        />
        <Route
          path="/bayar/:orderId"
          element={
            <Suspense fallback={<SetupFallback />}>
              <Pay />
            </Suspense>
          }
        />
        <Route
          path="/voucher/:orderId"
          element={
            <Suspense fallback={<SetupFallback />}>
              <Voucher />
            </Suspense>
          }
        />
        <Route
          path="/setup"
          element={
            <SetupGuard>
              <Suspense fallback={<SetupFallback />}>
                <Setup />
              </Suspense>
            </SetupGuard>
          }
        />
        <Route path="/admin/login" element={<AdminLogin />} />
        <Route
          path="/admin"
          element={
            <RequireAuth>
              <Suspense fallback={<AdminFallback />}>
                <AdminLayout />
              </Suspense>
            </RequireAuth>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="transaksi" element={<Transactions />} />
          <Route path="transaksi/:orderId" element={<TransactionDetail />} />
          <Route path="paket" element={<PackagesPage />} />
          <Route path="router" element={<RouterPage />} />
          <Route path="pengaturan" element={<SettingsPage />} />
        </Route>
        <Route path="*" element={<NotFound />} />
      </Routes>
    </MotionConfig>
  );
}
