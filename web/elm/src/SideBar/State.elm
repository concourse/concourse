module SideBar.State exposing (SideBarState, decodeSideBarState, encodeSideBarState)

import Json.Decode
import Json.Decode.Extra exposing (andMap)
import Json.Encode


type alias SideBarState =
    { isOpen : Bool
    , width : Float
    }


encodeSideBarState : SideBarState -> Json.Encode.Value
encodeSideBarState state =
    Json.Encode.object
        [ ( "is_open", state.isOpen |> Json.Encode.bool )
        , ( "width", state.width |> Json.Encode.float )
        ]


decodeSideBarState : Json.Decode.Decoder SideBarState
decodeSideBarState =
    Json.Decode.succeed SideBarState
        |> andMap (Json.Decode.field "is_open" Json.Decode.bool)
        |> andMap (Json.Decode.field "width" Json.Decode.float)
