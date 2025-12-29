import type { MouseEventHandler } from "react"
import clsx from "clsx"
import { useState } from "react"

const Button = ({ onClick }: { onClick: MouseEventHandler<HTMLDivElement> }) => {
  const [isToggled, setIsToggled] = useState(false)

  const handleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    setIsToggled(!isToggled)
    onClick(e)
  }

  return (
    <div
      className={clsx(
        "h-7 w-12 rounded-full bg-green flex items-center px-1 cursor-pointer transition-all duration-300",
        isToggled ? "justify-end" : "justify-start",
      )}
      onClick={handleClick}
    >
      <div className="size-5 rounded-full bg-black" />
    </div>
  )
}

export default Button
