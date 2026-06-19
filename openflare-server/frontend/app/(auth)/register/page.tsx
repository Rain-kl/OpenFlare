import {Suspense} from "react"
import {RegisterPage} from "@/components/auth/register-page"

export default function Page() {
  return (
    <Suspense>
      <RegisterPage />
    </Suspense>
  )
}
