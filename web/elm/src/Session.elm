module Session exposing (Session)

import Message.Message as Message
import SideBar.SideBar as SideBar
import UserState exposing (UserState)


type alias Session a =
    SideBar.Model
        { a
            | userState : UserState
            , hovered : Maybe Message.DomID
        }
