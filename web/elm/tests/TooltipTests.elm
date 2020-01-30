module TooltipTests exposing (all)

import Browser.Dom
import Common
import Expect
import HoverState exposing (TooltipPosition(..))
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Test exposing (Test, describe, test)
import Tooltip


all : Test
all =
    describe ".handleCallback" <|
        [ describe "OnlyShowWhenOverflowing policy"
            [ test "callback with overflowing viewport turns hover -> pending" <|
                \_ ->
                    ( { hovered = HoverState.Hovered domID }, [] )
                        |> Tooltip.handleCallback
                            (Callback.GotViewport domID Callback.OnlyShowWhenOverflowing <|
                                Ok overflowingViewport
                            )
                        |> Tuple.first
                        |> .hovered
                        |> Expect.equal (HoverState.TooltipPending domID)
            , test "callback with overflowing viewport gets element position" <|
                \_ ->
                    ( { hovered = HoverState.Hovered domID }, [] )
                        |> Tooltip.handleCallback
                            (Callback.GotViewport domID Callback.OnlyShowWhenOverflowing <|
                                Ok overflowingViewport
                            )
                        |> Tuple.second
                        |> Common.contains (Effects.GetElement domID)
            , test "callback with non-overflowing does nothing" <|
                \_ ->
                    ( { hovered = HoverState.Hovered domID }, [] )
                        |> Tooltip.handleCallback
                            (Callback.GotViewport domID Callback.OnlyShowWhenOverflowing <|
                                Ok nonOverflowingViewport
                            )
                        |> Tuple.first
                        |> .hovered
                        |> Expect.equal (HoverState.Hovered domID)
            ]
        , test "AlwaysShow callback with non-overflowing viewport gets element" <|
            \_ ->
                ( { hovered = HoverState.Hovered domID }, [] )
                    |> Tooltip.handleCallback
                        (Callback.GotViewport domID Callback.AlwaysShow <|
                            Ok nonOverflowingViewport
                        )
                    |> Tuple.second
                    |> Common.contains (Effects.GetElement domID)
        , test "callback with tooltip position turns pending -> tooltip" <|
            \_ ->
                ( { hovered = HoverState.TooltipPending domID }, [] )
                    |> Tooltip.handleCallback
                        (Callback.GotElement <| Ok elementPosition)
                    |> Tuple.first
                    |> .hovered
                    |> Expect.equal (HoverState.Tooltip domID (Top 0.5 1))
        ]


elementPosition : Browser.Dom.Element
elementPosition =
    { scene =
        { width = 0
        , height = 0
        }
    , viewport =
        { width = 0
        , height = 0
        , x = 0
        , y = 0
        }
    , element =
        { x = 0
        , y = 0
        , width = 1
        , height = 1
        }
    }


nonOverflowingViewport : Browser.Dom.Viewport
nonOverflowingViewport =
    { scene =
        { width = 1
        , height = 0
        }
    , viewport =
        { width = 1
        , height = 0
        , x = 0
        , y = 0
        }
    }


domID : DomID
domID =
    SideBarPipeline
        { teamName = "team"
        , pipelineName = "pipeline"
        }


overflowingViewport : Browser.Dom.Viewport
overflowingViewport =
    { scene =
        { width = 1
        , height = 0
        }
    , viewport =
        { width = 0
        , height = 0
        , x = 0
        , y = 0
        }
    }
