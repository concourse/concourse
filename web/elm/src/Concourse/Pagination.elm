module Concourse.Pagination exposing
    ( Direction(..)
    , Page
    , Paginated
    , Pagination
    , chevronContainer
    , chevronLeft
    , chevronRight
    , equal
    )

import Assets
import Colors
import Html
import Html.Attributes exposing (style)


type alias Paginated a =
    { content : List a
    , pagination : Pagination
    }


type alias Pagination =
    { previousPage : Maybe Page
    , nextPage : Maybe Page
    }


type alias Page =
    { direction : Direction
    , limit : Int
    }


type Direction
    = From Int
    | To Int
    | ToMostRecent



--directionEqual : Direction -> Direction -> Bool
--directionEqual d1 d2 =
--    case ( d1, d2 ) of
--        ( From p1, From p2 ) ->
--            p1 == p2
--
--        ( To p1, To p2 ) ->
--            p1 == p2
--
--        ( _, _ ) ->
--            False


equal : Page -> Page -> Bool
equal =
    (==)


chevronContainer : List (Html.Attribute msg)
chevronContainer =
    [ style "padding" "5px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "border-left" <| "1px solid " ++ Colors.background
    ]


chevron :
    Assets.Asset
    -> { enabled : Bool, hovered : Bool }
    -> List (Html.Attribute msg)
chevron asset { enabled, hovered } =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just asset
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "width" "24px"
    , style "height" "24px"
    , style "padding" "5px"
    , style "opacity" <|
        if enabled then
            "1"

        else
            "0.5"
    ]
        ++ (if hovered then
                [ style "background-color" Colors.paginationHover
                , style "border-radius" "50%"
                ]

            else
                []
           )


chevronLeft : { enabled : Bool, hovered : Bool } -> List (Html.Attribute msg)
chevronLeft =
    chevron <| Assets.ChevronLeft


chevronRight : { enabled : Bool, hovered : Bool } -> List (Html.Attribute msg)
chevronRight =
    chevron <| Assets.ChevronRight
