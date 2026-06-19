import {redirect} from "next/navigation"

export default function SecurityPage() {
  redirect("/admin/settings")
}
