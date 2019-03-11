module Concourse.Pagination exposing
    ( Direction(..)
    , Page
    , Paginated
    , Pagination
    , chevron
    , chevronContainer
    , equal
    )

import Colors


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
    = Since Int
    | Until Int
    | From Int
    | To Int


directionEqual : Direction -> Direction -> Bool
directionEqual d1 d2 =
    case ( d1, d2 ) of
        ( Since p1, Since p2 ) ->
            p1 == p2

        ( Until p1, Until p2 ) ->
            p1 == p2

        ( From p1, From p2 ) ->
            p1 == p2

        ( To p1, To p2 ) ->
            p1 == p2

        ( _, _ ) ->
            False


equal : Page -> Page -> Bool
equal one two =
    directionEqual one.direction two.direction


chevronContainer : List ( String, String )
chevronContainer =
    [ ( "padding", "5px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "border-left", "1px solid " ++ Colors.background )
    ]


chevron :
    { direction : String, enabled : Bool, hovered : Bool }
    -> List ( String, String )
chevron { direction, enabled, hovered } =
    [ ( "background-image"
      , "url(/public/images/baseline-chevron-" ++ direction ++ "-24px.svg)"
      )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "width", "24px" )
    , ( "height", "24px" )
    , ( "padding", "5px" )
    , ( "opacity"
      , if enabled then
            "1"

        else
            "0.5"
      )
    ]
        ++ (if hovered then
                [ ( "background-color", Colors.paginationHover )
                , ( "border-radius", "50%" )
                ]

            else
                []
           )
