import os
import math
import random
import sys

import matplotlib
matplotlib.use("TkAgg")

from matplotlib           import pyplot as plt
from matplotlib.animation import FuncAnimation
from matplotlib.widgets   import Slider

import pydot
from fpdf import FPDF
from scipy.misc import imread

def anim(num_frames, redraw_func):
    """Figure with horizontal scrollbar and play capabilities
    """
    fig_handle = plt.figure()
  
    axes_handle = plt.axes([0, 0.03, 1, 0.97])
    axes_handle.set_axis_off()

    scroll_axes_handle = plt.axes([0, 0, 1, 0.03], facecolor='lightgoldenrodyellow')
    scroll_handle      = Slider(scroll_axes_handle, '', 0.0, num_frames - 1, valinit=0.0)

    def draw_new(_):
      plt.sca(axes_handle)
      redraw_func(int(scroll_handle.val), axes_handle)
      fig_handle.canvas.draw_idle()

    def scroll(new_f):
      new_f = min(max(new_f, 0), num_frames - 1) 
      cur_f = scroll_handle.val

      if new_f == (num_frames - 1):
        play.running = False

      if cur_f != new_f:
        scroll_handle.set_val(new_f)

      return axes_handle

    def play():
      play.running ^= True 
      if play.running:
        frame_idxs = range(int(scroll_handle.val), num_frames)
        play.anim  = FuncAnimation(fig_handle, scroll, frame_idxs,
                                   interval=100000000000, repeat=False,
                                   blit=False)
        plt.draw()
      else:
        play.anim.event_source.stop()

    play.running = False

    def key_press(event):
      key = event.key
      f = scroll_handle.val
      if key == 'left':
        scroll(f - 1)
      elif key == 'right':
        scroll(f + 1)
      elif key == 'q':
        sys.exit(0)

    scroll_handle.on_changed(draw_new)
    fig_handle.canvas.mpl_connect('key_press_event', key_press)
    fig_handle.canvas.get_tk_widget().focus_force() 

    redraw_func(0, axes_handle)
    play()
    plt.show()
  
def makeGraph(n, nNodes, connections, messages, label):
    filename = "out%d"%n
    file = open("%s.dot"%filename, "w")
    file.write("digraph G {\nlayout=\"neato\"\n")
    file.write("graph [dpi = 300];")

    radius = nNodes*0.2

    for i in range(1, nNodes+1):
        angle = 2*math.pi*i/nNodes
        x = radius*math.cos(angle)
        y = radius*math.sin(angle)

        file.write("  %d[pos=\"%f,%f!\",shape=circle];\n"%(i-1, x, y))

    for i in range(len(connections)):
        for j in connections[i]:
            if j not in messages[i]: file.write("%d->%d\n"%(i, j))
        for j in messages[i]:
            file.write("%d->%d[color=\"red\"];\n"%(i, j))

    file.write("labelloc=\"t\";\n")
    file.write("label=\"%s\";\n"%label)
    file.write("}\n")
    file.close()
    
    (graph,) = pydot.graph_from_dot_file("%s.dot"%filename)
    graph.write_png('%s.png'%filename)

def makeAnim(img_files):
    def redraw_fn(f, axes):
        img_file = img_files[f]
        img = imread(img_file)
        if not hasattr(redraw_fn, "initialized") or not redraw_fn.initialized:
            redraw_fn.im = axes.imshow(img, animated=True)
            redraw_fn.initialized = True
        else:
            redraw_fn.im.set_array(img)
        redraw_fn.initialized = False

    anim(len(img_files), redraw_fn)
    
if __name__=="__main__":
    makeGraph(0, 5, [[random.randint(0, 4) for i in range(2)] for j in range(5)], [[random.randint(0, 4)] for j in range(5)])
    makeGraph(1, 5, [[random.randint(0, 4) for i in range(2)] for j in range(5)], [[random.randint(0, 4)] for j in range(5)])
    makeGraph(2, 5, [[random.randint(0, 4) for i in range(2)] for j in range(5)], [[random.randint(0, 4)] for j in range(5)])
    makeGraph(3, 5, [[random.randint(0, 4) for i in range(2)] for j in range(5)], [[random.randint(0, 4)] for j in range(5)])
    makeGraph(4, 5, [[random.randint(0, 4) for i in range(2)] for j in range(5)], [[random.randint(0, 4)] for j in range(5)])
    makeAnim(["out%d.png"%i for i in range(0, 5)])
    os.system("rm -f out*.dot out*.png")
