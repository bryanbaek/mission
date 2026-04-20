import { NavLink, Outlet } from "react-router-dom";
import { UserButton } from "@clerk/clerk-react";

const nav = [
  { to: "/", label: "Tenants" },
  { to: "/agents", label: "Agents" },
  { to: "/queries", label: "Queries" },
];

export default function Layout() {
  return (
    <main className="min-h-screen bg-slate-100 p-6 text-slate-900">
      <div className="mx-auto flex max-w-6xl flex-col gap-6">
        <header
          className={[
            "flex items-center justify-between rounded-3xl",
            "border border-slate-200 bg-white px-6 py-3 shadow-sm",
          ].join(" ")}
        >
          <nav className="flex items-center gap-2">
            {nav.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  [
                    "rounded-xl px-3 py-1.5 text-sm font-medium",
                    isActive
                      ? "bg-slate-950 text-white"
                      : "text-slate-600 hover:bg-slate-100",
                  ].join(" ")
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          <UserButton afterSignOutUrl="/" />
        </header>
        <Outlet />
      </div>
    </main>
  );
}
